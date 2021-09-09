package logs

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils/trace"
)

// LogStreamer --
type LogStreamer struct {
	ctx             context.Context
	jobTrace        trace.JobTrace
	queueClient     queue.Client
	logTopic        string
	userID          string
	requestID       uint64
	jobType         string
	taskType        string
	jobExecutionID  string
	taskExecutionID string
	antID           string
	maxMessageSize  int
}

// NewLogStreamer --
func NewLogStreamer(
	ctx context.Context,
	antCfg *config.AntConfig,
	taskReq *types.TaskRequest,
	queueClient queue.Client,
) (streamer *LogStreamer, err error) {
	streamer = &LogStreamer{
		ctx:             ctx,
		queueClient:     queueClient,
		logTopic:        antCfg.GetLogTopic(),
		userID:          taskReq.UserID,
		requestID:       taskReq.JobRequestID,
		jobType:         taskReq.JobType,
		taskType:        taskReq.TaskType,
		jobExecutionID:  taskReq.JobExecutionID,
		taskExecutionID: taskReq.TaskExecutionID,
		antID:           antCfg.ID,
		maxMessageSize:  antCfg.MaxStreamingLogMessageSize,
	}
	masks := []string{
		"AWS_ENDPOINT",
		"AWS_ACCESS_KEY_ID",
		"AWS_URL",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_DEFAULT_REGION",
		antCfg.S3.Endpoint,
		antCfg.S3.AccessKeyID,
		antCfg.S3.SecretAccessKey,
		antCfg.S3.Region,
	}
	masks = append(masks, taskReq.GetMaskFields()...)
	streamer.jobTrace, err = trace.NewJobTrace(
		streamer.publish,
		antCfg.OutputLimit,
		masks)
	if err != nil {
		return nil, fmt.Errorf("failed to create job trace due to %v", err)
	}
	return streamer, nil
}

// Writeln writes string and new-line to the Buffer --
func (s *LogStreamer) Writeln(input string) (n int, err error) {
	n, err = s.jobTrace.Writeln(input)
	if err != nil {
		return 0, err
	}
	// will be called by line feeder
	//s.publish([]byte(input + "\n"))

	return n, err
}

// Write Data to the Buffer
func (s *LogStreamer) Write(data []byte) (n int, err error) {
	n, err = s.jobTrace.Write(data)
	if err != nil {
		return 0, err
	}
	// will be called by line feeder
	//s.publish(data)

	return n, err
}

func (s *LogStreamer) publish(data []byte) {
	if len(data) == 0 {
		return
	}
	var msg string
	if len(data) > s.maxMessageSize {
		msg = fmt.Sprintf("%s\n__TRUNCATED__\n%s",
			data[0:s.maxMessageSize/2], data[s.maxMessageSize/2:])
	} else {
		msg = string(data)
	}

	event := events.NewLogEvent(
		"LogStreamer",
		s.userID,
		s.requestID,
		s.jobType,
		s.taskType,
		s.jobExecutionID,
		s.taskExecutionID,
		msg,
		s.antID)

	if b, serErr := event.Marshal(); serErr != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "LogStreamer",
			"Message":   string(data),
			"Error":     serErr,
		}).Error("failed to marshal log event")
	} else {
		if _, pubErr := s.queueClient.Publish(s.ctx, s.logTopic, make(map[string]string), b, false); pubErr != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "LogStreamer",
				"Message":   string(data),
				"Error":     pubErr,
			}).Error("failed to publish log event")
		}
	}
}

// Finish closes writer and returns contents
func (s *LogStreamer) Finish() ([]byte, error) {
	return s.jobTrace.Finish()
}

// Close closes buffer
func (s *LogStreamer) Close() {
	s.jobTrace.Close()
}
