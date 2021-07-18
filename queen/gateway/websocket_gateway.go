package gateway

import (
	"context"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/queen/repository"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
)

var (
	upgrader = websocket.Upgrader{}
)

// Gateway for websockets
type Gateway struct {
	id           string
	serverCfg    *config.ServerConfig
	queueClient  queue.Client
	logsArchiver repository.LogEventRepository
	registry     *LeaseRegistry
	ticker       *time.Ticker
}

// New creates new gateway for routing events to websocket clients
func New(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	logsArchiver repository.LogEventRepository,
	webserver web.Server) *Gateway {
	gw := &Gateway{
		id:           serverCfg.ID + "-websocket-gateway",
		serverCfg:    serverCfg,
		queueClient:  queueClient,
		logsArchiver: logsArchiver,
		registry:     NewLeaseRegistry(),
	}
	webserver.GET("/ws/subscriptions", gw.Register, acl.New(acl.Websocket, acl.Subscribe)).Name = "websocket_subscription"
	return gw
}

// Start - creates periodic ticker for scheduling pending jobs
func (gw *Gateway) Start(ctx context.Context) (err error) {
	if gw.serverCfg.GatewaySubscriptions["JobDefinitionLifecycleEvent"] {
		if err = gw.subscribeToJobDefinitionLifecycleEvent(
			ctx,
			gw.serverCfg.GetJobDefinitionLifecycleTopic()); err != nil {
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["JobRequestLifecycleEvent"] {
		if err = gw.subscribeToJobRequestLifecycleEvent(
			ctx,
			gw.serverCfg.GetJobRequestLifecycleTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["JobExecutionLifecycleEvent"] {
		if err = gw.subscribeToJobExecutionLifecycleEvent(
			ctx,
			gw.serverCfg.GetJobExecutionLifecycleTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["TaskExecutionLifecycleEvent"] {
		if err = gw.subscribeToTaskExecutionLifecycleEvent(
			ctx,
			gw.serverCfg.GetTaskExecutionLifecycleTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["ContainersLifecycleEvents"] {
		if err = gw.subscribeToContainersLifecycleEvents(
			ctx,
			gw.serverCfg.GetContainerLifecycleTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["LogEvent"] {
		if err = gw.subscribeToLogEvent(
			ctx,
			gw.serverCfg.GetLogTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["HealthErrorEvent"] {
		if err = gw.subscribeToHealthErrorEvent(
			ctx,
			gw.serverCfg.GetHealthErrorTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	gw.startReaperTicker(ctx)
	return nil
}

// Stop - stops background subscription and ticker routine
func (gw *Gateway) Stop(ctx context.Context) error {
	err1 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetJobDefinitionLifecycleTopic(),
		gw.id,
	)
	err2 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetJobRequestLifecycleTopic(),
		gw.id,
	)
	err3 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetJobExecutionLifecycleTopic(),
		gw.id,
	)
	err4 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetTaskExecutionLifecycleTopic(),
		gw.id,
	)
	err5 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetContainerLifecycleTopic(),
		gw.id,
	)
	err6 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetLogTopic(),
		gw.id,
	)
	err7 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetHealthErrorTopic(),
		gw.id,
	)
	gw.ticker.Stop()
	return utils.ErrorsAny(err1, err2, err3, err4, err5, err6, err7)
}

// Register websocket subscription
func (gw *Gateway) Register(c web.WebContext) (err error) {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	go gw.handleRegistration(c, ws)
	return
}

func (gw *Gateway) handleRegistration(c web.WebContext, ws *websocket.Conn) {
	defer func() {
		for _, lease := range gw.registry.getLeasesByAddress(ws.RemoteAddr().String()) {
			_ = gw.registry.Remove(lease)
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketGateway",
				"Address":   ws.RemoteAddr().String(),
				"Key":       lease.Key(),
			}).Info("removing disconnected lease")
		}
	}()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if !strings.Contains(err.Error(), "close 1001") {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketGateway",
					"Address":   ws.RemoteAddr().String(),
					"Error":     err,
				}).Warnf("failed to receive websocket message")
			}
			return
		}

		lease, err := UnmarshalSubscriptionLease(msg, ws)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketGateway",
				"Address":   ws.RemoteAddr().String(),
				"Error":     err,
			}).Warnf("failed to unmarshal websocket message")
			return
		}

		if gw.serverCfg.Auth.Enabled {
			user := web.GetDBLoggedUserFromSession(c)
			if user == nil {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketGateway",
					"Address":   ws.RemoteAddr().String(),
					"Lease":     lease,
				}).Errorf("failed to get logged in user")
				_ = lease.WriteMessage(events.NewErrorEvent("WebsocketGateway", "", "session token not found").Marshal())
				return
			}
			lease.userID = user.ID
		}

		if err := gw.registry.Add(lease); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketGateway",
				"Address":   ws.RemoteAddr().String(),
				"Error":     err,
			}).Warnf("failed to add lease")
			return
		}
	}
}
