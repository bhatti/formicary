package webhook

import (
	"context"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
)

// Processor for websockets
type Processor struct {
	serverCfg                          *config.ServerConfig
	queueClient                        queue.Client
	http                               web.HTTPClient
	jobWebhookLifecycleSubscriptionID  string
	taskWebhookLifecycleSubscriptionID string
	jobsProcessed                      int64
	tasksProcessed                     int64
}

// New creates new gateway for routing events to websocket clients
func New(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	http web.HTTPClient,
) *Processor {
	return &Processor{
		serverCfg:   serverCfg,
		queueClient: queueClient,
		http:        http,
	}
}

// Start - creates periodic ticker for scheduling pending jobs
func (p *Processor) Start(ctx context.Context) (err error) {
	if p.jobWebhookLifecycleSubscriptionID, err = p.subscribeToJobWebhookLifecycleEvent(
		ctx,
		p.serverCfg.Common.GetJobWebhookTopic()); err != nil {
		_ = p.Stop(ctx)
		return err
	}
	if p.taskWebhookLifecycleSubscriptionID, err = p.subscribeToTaskWebhookLifecycleEvent(
		ctx,
		p.serverCfg.Common.GetTaskWebhookTopic()); err != nil {
		_ = p.Stop(ctx)
		return err
	}
	return nil
}

// Stop - stops background subscription and ticker routine
func (p *Processor) Stop(ctx context.Context) error {
	err1 := p.queueClient.UnSubscribe(
		ctx,
		p.serverCfg.Common.GetJobWebhookTopic(),
		p.jobWebhookLifecycleSubscriptionID)
	err2 := p.queueClient.UnSubscribe(
		ctx,
		p.serverCfg.Common.GetTaskWebhookTopic(),
		p.taskWebhookLifecycleSubscriptionID)
	return utils.ErrorsAny(err1, err2)
}
