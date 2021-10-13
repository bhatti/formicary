package golang

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"time"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/types"
)

// MessagingHandler structure
type MessagingHandler struct {
	id            string
	subscriberID  string
	requestTopic  string
	responseTopic string
	queueClient   queue.Client
}

// NewMessagingHandler constructor
func NewMessagingHandler(
	id string,
	requestTopic string,
	responseTopic string,
	queueClient queue.Client,
) *MessagingHandler {
	return &MessagingHandler{
		id:            id,
		requestTopic:  requestTopic,
		responseTopic: responseTopic,
		queueClient:   queueClient,
	}
}

// Start starts subscription
func (h *MessagingHandler) Start(
	ctx context.Context,
) (err error) {
	if h.id == "" {
		return fmt.Errorf("id is not specified")
	}
	if h.requestTopic == "" {
		return fmt.Errorf("requestTopic is not specified")
	}
	if h.responseTopic == "" {
		return fmt.Errorf("responseTopic is not specified")
	}
	h.subscriberID, err = h.queueClient.Subscribe(
		ctx,
		h.requestTopic,
		true, // shared subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			err = h.execute(ctx, event.Properties, event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "MessagingHandler",
					"Payload":   string(event.Payload),
					"Target":    h.id,
					"Error":     err}).Error("failed to execute")
				return err
			}
			return nil
		},
		make(map[string]string),
	)
	return
}

// Stop stops subscription
func (h *MessagingHandler) Stop(
	ctx context.Context,
) (err error) {
	return h.queueClient.UnSubscribe(
		ctx,
		h.requestTopic,
		h.subscriberID)
}

// execute incoming request
func (h *MessagingHandler) execute(
	ctx context.Context,
	props queue.MessageHeaders,
	reqPayload []byte) (err error) {
	var req types.TaskRequest
	err = json.Unmarshal(reqPayload, &req)
	if err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"ID":           h.id,
		"RequestTopic": h.requestTopic,
		"Request":      req.String(),
	}).
		Infof("received messaging request")
	resp := types.NewTaskResponse(&req)

	// Implement business logic below
	epoch := time.Now().Unix()
	if epoch%2 == 0 {
		resp.Status = types.COMPLETED
	} else {
		resp.ErrorCode = "ERR_MESSAGING_WORKER"
		resp.ErrorMessage = "mock error for messaging client"
		resp.Status = types.FAILED
	}
	resp.AddContext("epoch", epoch)

	// Send back reply
	resPayload, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = h.queueClient.Send(
		ctx,
		h.responseTopic,
		resPayload,
		queue.NewMessageHeaders(
			queue.ReusableTopicKey, "false",
			queue.CorrelationIDKey, props.GetCorrelationID(),
		),
	)
	logrus.WithFields(logrus.Fields{
		"ID":            h.id,
		"RequestTopic":  h.requestTopic,
		"ResponseTopic": h.responseTopic,
		"Status":        resp.Status,
	}).
		Infof("sent reply")
	return err
}
