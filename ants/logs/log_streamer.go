package logs

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/ant_config"
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
	requestID       string
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
	antCfg *ant_config.AntConfig,
	taskReq *types.TaskRequest,
	queueClient queue.Client,
) (streamer *LogStreamer, err error) {
	streamer = &LogStreamer{
		ctx:             ctx,
		queueClient:     queueClient,
		logTopic:        antCfg.Common.GetLogTopic(),
		userID:          taskReq.UserID,
		requestID:       taskReq.JobRequestID,
		jobType:         taskReq.JobType,
		taskType:        taskReq.TaskType,
		jobExecutionID:  taskReq.JobExecutionID,
		taskExecutionID: taskReq.TaskExecutionID,
		antID:           antCfg.Common.ID,
		maxMessageSize:  antCfg.Common.MaxStreamingLogMessageSize,
	}
	masks := []string{
		"AWS_ENDPOINT",
		"AWS_ACCESS_KEY_ID",
		"AWS_URL",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_DEFAULT_REGION",
		antCfg.Common.S3.Endpoint,
		antCfg.Common.S3.AccessKeyID,
		antCfg.Common.S3.SecretAccessKey,
		antCfg.Common.S3.Region,
	}
	masks = append(masks, taskReq.GetMaskFields()...)
	streamer.jobTrace, err = trace.NewJobTrace(
		streamer.publish,
		antCfg.OutputLimit,
		masks)
	if err != nil {
		return nil, fmt.Errorf("failed to create job trace due to %w", err)
	}
	return streamer, nil
}

// Writeln Data to the Buffer
func (s *LogStreamer) Writeln(data string, tags string) (n int, err error) {
	return s.jobTrace.Writeln(data, tags)
}

// Write Data to the Buffer
func (s *LogStreamer) Write(data []byte, tags string) (n int, err error) {
	return s.jobTrace.Write(data, tags)
}

func (s *LogStreamer) publish(data []byte, tags string) {
	if len(data) == 0 {
		return
	}
	if s.ctx.Err() != nil {
		return
	}
	var msg string
	if len(data) > s.maxMessageSize {
		msg = fmt.Sprintf("%s\n__TRUNCATED__(%d:%d)\n%s",
			data[0:s.maxMessageSize/2], len(data), s.maxMessageSize, data[s.maxMessageSize/2:])
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
		tags,
		s.antID)

	if b, serErr := event.Marshal(); serErr != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "LogStreamer",
			"Message":   string(data),
			"Tags":      tags,
			"Error":     serErr,
		}).Error("failed to marshal log event")
	} else {
		if _, pubErr := s.queueClient.Publish(
			s.ctx,
			s.logTopic,
			b,
			queue.NewMessageHeaders(
				queue.DisableBatchingKey, "true",
				"RequestID", s.requestID,
				"UserID", s.userID,
			),
		); pubErr != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "LogStreamer",
				"Message":   string(data),
				"Tags":      tags,
			}).WithError(pubErr).Error("failed to publish log event")
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
