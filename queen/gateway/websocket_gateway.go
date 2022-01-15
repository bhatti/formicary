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
	id                                   string
	serverCfg                            *config.ServerConfig
	queueClient                          queue.Client
	logsArchiver                         repository.LogEventRepository
	registry                             *LeaseRegistry
	ticker                               *time.Ticker
	jobDefinitionLifecycleSubscriptionID string
	jobRequestLifecycleSubscriptionID    string
	jobExecutionLifecycleSubscriptionID  string
	taskExecutionLifecycleSubscriptionID string
	containersLifecycleSubscriptionID    string
	logsSubscriptionID                   string
	healthErrorSubscriptionID            string
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
	webserver.GET("/ws/subscriptions", gw.Register, acl.NewPermission(acl.Websocket, acl.Subscribe)).Name = "websocket_subscription"
	return gw
}

// Start - creates periodic ticker for scheduling pending jobs
func (gw *Gateway) Start(ctx context.Context) (err error) {
	if gw.serverCfg.GatewaySubscriptions["JobDefinitionLifecycleEvent"] {
		if gw.jobDefinitionLifecycleSubscriptionID, err = gw.subscribeToJobDefinitionLifecycleEvent(
			ctx,
			gw.serverCfg.GetJobDefinitionLifecycleTopic()); err != nil {
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["JobRequestLifecycleEvent"] {
		if gw.jobRequestLifecycleSubscriptionID, err = gw.subscribeToJobRequestLifecycleEvent(
			ctx,
			gw.serverCfg.GetJobRequestLifecycleTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["JobExecutionLifecycleEvent"] {
		if gw.jobExecutionLifecycleSubscriptionID, err = gw.subscribeToJobExecutionLifecycleEvent(
			ctx,
			gw.serverCfg.GetJobExecutionLifecycleTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["TaskExecutionLifecycleEvent"] {
		if gw.taskExecutionLifecycleSubscriptionID, err = gw.subscribeToTaskExecutionLifecycleEvent(
			ctx,
			gw.serverCfg.GetTaskExecutionLifecycleTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["ContainersLifecycleEvents"] {
		if gw.containersLifecycleSubscriptionID, err = gw.subscribeToContainersLifecycleEvents(
			ctx,
			gw.serverCfg.GetContainerLifecycleTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["LogEvent"] {
		if gw.logsSubscriptionID, err = gw.subscribeToLogEvent(
			ctx,
			gw.serverCfg.GetLogTopic()); err != nil {
			_ = gw.Stop(ctx)
			return err
		}
	}
	if gw.serverCfg.GatewaySubscriptions["HealthErrorEvent"] {
		if gw.healthErrorSubscriptionID, err = gw.subscribeToHealthErrorEvent(
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
		gw.jobDefinitionLifecycleSubscriptionID)
	err2 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetJobRequestLifecycleTopic(),
		gw.jobRequestLifecycleSubscriptionID)
	err3 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetJobExecutionLifecycleTopic(),
		gw.jobExecutionLifecycleSubscriptionID)
	err4 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetTaskExecutionLifecycleTopic(),
		gw.taskExecutionLifecycleSubscriptionID)
	err5 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetContainerLifecycleTopic(),
		gw.containersLifecycleSubscriptionID)
	err6 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetLogTopic(),
		gw.logsSubscriptionID)
	err7 := gw.queueClient.UnSubscribe(
		ctx,
		gw.serverCfg.GetHealthErrorTopic(),
		gw.healthErrorSubscriptionID)
	gw.ticker.Stop()
	return utils.ErrorsAny(err1, err2, err3, err4, err5, err6, err7)
}

// Register websocket subscription
func (gw *Gateway) Register(c web.APIContext) (err error) {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	go gw.handleRegistration(c, ws)
	return
}

func (gw *Gateway) handleRegistration(c web.APIContext, ws *websocket.Conn) {
	defer func() {
		for _, lease := range gw.registry.getLeasesByAddress(ws.RemoteAddr().String()) {
			_ = gw.registry.Remove(lease)
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketGateway",
					"Address":   ws.RemoteAddr().String(),
					"Key":       lease.Key(),
				}).Debugf("removing disconnected lease")
			}
		}
	}()

	controlTries := 0
	for {
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			if !strings.Contains(err.Error(), "close 1001") {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketGateway",
					"Address":   ws.RemoteAddr().String(),
					"MsgType":   msgType,
					"Error":     err,
				}).Warnf("failed to receive websocket message from web client")
			}
			return
		}
		if controlTries < 10 && msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketGateway",
				"Address":   ws.RemoteAddr().String(),
				"MsgType":   msgType,
				"Data":      string(msg),
			}).Warnf("received control websocket message from web client")
			controlTries++
			continue
		} else if controlTries >= 10 {
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketGateway",
				"Address":   ws.RemoteAddr().String(),
				"MsgType":   msgType,
				"Data":      string(msg),
			}).Warnf("received too many control websocket message from web client")
			return
		}
		controlTries = 0

		lease, err := UnmarshalSubscriptionLease(msg, ws)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketGateway",
				"Address":   ws.RemoteAddr().String(),
				"Error":     err,
			}).Warnf("failed to unmarshal websocket message from web client")
			return
		}

		if gw.serverCfg.Auth.Enabled {
			user := web.GetDBLoggedUserFromSession(c)
			if user == nil {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketGateway",
					"Address":   ws.RemoteAddr().String(),
					"Lease":     lease,
				}).Errorf("failed to get logged in user from web client")
				msg := events.NewErrorEvent("WebsocketGateway", "", "session token not found").Marshal()
				_ = lease.connection.WriteMessage(websocket.TextMessage, msg)
				return
			}
			lease.userID = user.ID
		}

		if err := gw.registry.Add(lease); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketGateway",
				"Address":   ws.RemoteAddr().String(),
				"Error":     err,
			}).Warnf("failed to add lease from web client")
			return
		}
	}
}
