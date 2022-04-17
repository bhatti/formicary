package wstask

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/resource"
	"strings"
	"sync"
	"time"
)

// WebsocketTasklet  keeps track of subscriptions
type WebsocketTasklet struct {
	*tasklet.BaseTasklet
	id              string
	user            *types.User
	resourceManager resource.Manager
	registration    *types.AntRegistration
	connection      *websocket.Conn
	errorHandler    IOErrorHandler
	ticker          *time.Ticker
	closed          bool
	lock            sync.RWMutex // this protects writes to websockets because only one goroutine can write at a time
}

// NewWebsocketTasklet constructor
func NewWebsocketTasklet(
	ctx context.Context,
	serverCfg *config.ServerConfig,
	user *types.User,
	resourceManager resource.Manager,
	requestRegistry tasklet.RequestRegistry,
	queueClient queue.Client,
	requestTopic string,
	connection *websocket.Conn,
	errorHandler IOErrorHandler) (wc *WebsocketTasklet, err error) {
	wc = &WebsocketTasklet{
		user:            user,
		resourceManager: resourceManager,
		connection:      connection,
		errorHandler:    errorHandler,
	}
	err = wc.receiveRegistration(ctx, requestTopic)
	if err != nil {
		return nil, err
	}
	wc.setupPingTicker(ctx)
	wc.BaseTasklet = tasklet.NewBaseTasklet(
		wc.registration.AntID,
		&serverCfg.CommonConfig,
		queueClient,
		func(ctx context.Context, event *queue.MessageEvent) bool {
			matched := event.Properties[queue.MessageTarget] == wc.registration.AntID
			if !matched && logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketTasklet",
					"Address":   connection.RemoteAddr().String(),
					"AntID":     wc.registration.AntID,
					"Header":    event.Properties,
					"User":      user,
					"Body":      string(event.Payload),
				}).Debugf("websocket-request-filtering")
			}
			return matched
		},
		requestRegistry,
		requestTopic,
		serverCfg.GetRegistrationTopic(),
		wc.registration,
		wc,
	)
	logrus.WithFields(logrus.Fields{
		"Component": "WebsocketTasklet",
		"User":      user,
		"Address":   connection.RemoteAddr().String(),
		"AntID":     wc.registration.AntID,
	}).Infof("registered websocket ant worker")
	return wc, err
}

// TerminateContainer terminates container
func (t *WebsocketTasklet) TerminateContainer(
	_ context.Context,
	_ *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	return nil, fmt.Errorf("cannot terminate container")
}

// ListContainers list containers
func (t *WebsocketTasklet) ListContainers(
	_ context.Context,
	req *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	taskResp = types.NewTaskResponse(req)
	taskResp.Status = types.COMPLETED
	taskResp.AddContext("containers", make([]*events.ContainerLifecycleEvent, 0))
	return
}

// PreExecute check if request can be executed
func (t *WebsocketTasklet) PreExecute(
	_ context.Context,
	_ *types.TaskRequest) bool {
	return true
}

// Execute task request
func (t *WebsocketTasklet) Execute(
	_ context.Context,
	taskReq *types.TaskRequest) (taskResp *types.TaskResponse, err error) {
	reqBody, err := taskReq.Marshal(t.registration.EncryptionKey)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}
	logrus.WithFields(logrus.Fields{
		"Component": "WebsocketTasklet",
		"Address":   t.connection.RemoteAddr().String(),
		"AntID":     t.registration.AntID,
		"Request":   taskReq,
	}).Infof("writing to remote websocket ant worker")
	err = t.write(reqBody)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "WebsocketTasklet",
			"Address":   t.connection.RemoteAddr().String(),
			"AntID":     t.registration.AntID,
			"Request":   string(reqBody),
			"Error":     err,
		}).Warnf("error writing to remote websocket ant worker")
		return taskReq.ErrorResponse(err), nil
	}
	resBody, err := t.read(0)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}

	taskResp, err = types.UnmarshalTaskResponse(t.registration.EncryptionKey, resBody)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "WebsocketTasklet",
			"AntID":     t.registration.AntID,
			"Address":   t.connection.RemoteAddr().String(),
			"Response":  string(resBody),
		}).Debugf("received response from remote websocket ant worker")
	}
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (t *WebsocketTasklet) write(payload []byte) (err error) {
	t.lock.Lock()
	err = t.connection.WriteMessage(websocket.TextMessage, payload)
	t.lock.Unlock()
	if err != nil {
		t.errorHandler(t.id)
	} else if t.registration != nil {
		t.registration.ReceivedAt = time.Now()
	}
	return
}

func (t *WebsocketTasklet) read(tries int) (data []byte, err error) {
	var msgType int
	t.lock.Lock()
	msgType, data, err = t.connection.ReadMessage()
	t.lock.Unlock()
	if err != nil {
		t.errorHandler(t.id)
	} else if t.registration != nil {
		t.registration.ReceivedAt = time.Now()
	}
	if tries < 10 && msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
		return t.read(tries + 1)
	} else if tries >= 10 {
		logrus.WithFields(logrus.Fields{
			"Component": "WebsocketTasklet",
			"Address":   t.connection.RemoteAddr().String(),
			"MsgType":   msgType,
			"Msg":       string(data),
		}).Warnf("received too many control message from ant worker, closing..")
		t.errorHandler(t.id)
	}

	return
}

func (t *WebsocketTasklet) receiveRegistration(
	ctx context.Context,
	requestTopic string,
) (err error) {
	msg, err := t.read(0)
	if err != nil {
		if !strings.Contains(err.Error(), "close 1001") {
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketTasklet",
				"Address":   t.connection.RemoteAddr().String(),
				"Error":     err,
			}).Warnf("failed to receive websocket message from ant worker")
		}
		return err
	}
	t.registration, err = types.UnmarshalAntRegistration(msg)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component":    "WebsocketTasklet",
			"registration": t.registration,
			"Address":      t.connection.RemoteAddr().String(),
			"Error":        err,
		}).Warnf("failed to unmarshal websocket message")
		return err
	}
	// override ant-id because we can't trust javascript clients
	t.id = t.connection.RemoteAddr().String() + ":" + t.registration.Key()
	t.registration.AntID = t.id
	t.registration.AntTopic = requestTopic
	t.registration.AutoRefresh = false
	t.registration.MaxCapacity = 1
	t.registration.Methods = []types.TaskMethod{types.WebSocket}
	t.registration.ValidRegistration = func(ctx context.Context) error {
		return t.ping()
	}
	if err = t.resourceManager.Register(ctx, t.registration); err != nil {
		return err
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":    "WebsocketTasklet",
			"registration": t.registration,
			"Address":      t.connection.RemoteAddr().String(),
			"id":           t.id,
		}).Debugf("websocket ant registration for tasklet")
	}
	return nil
}

// Close closes connection
func (t *WebsocketTasklet) Close(ctx context.Context) {
	t.lock.Lock()
	defer t.lock.Unlock()
	defer func() {
		if r := recover(); r != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "WebsocketTasklet",
				"t":         t,
				"Recover":   r,
			}).Error("recovering from panic when closing channel")
		}
	}()
	var unregistered bool
	var antID string
	if t.registration != nil {
		antID = t.registration.AntID
		unregistered, _ = t.resourceManager.Unregister(context.Background(), t.registration.AntID)
	}
	_ = t.connection.Close()
	_ = t.Stop(ctx)
	t.closed = true
	logrus.WithFields(logrus.Fields{
		"Component":    "WebsocketTasklet",
		"Registration": t.registration,
		"AntID":        antID,
		"Unregistered": unregistered,
	}).Info("removing websocket ant worker and unsubscribing")
}

func (t *WebsocketTasklet) isClosed() bool {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.closed
}

func (t *WebsocketTasklet) ping() error {
	expiration := time.Now().Add(time.Millisecond * 1000)
	if err := t.connection.WriteControl(websocket.PingMessage, []byte("ping"), expiration); err != nil && !strings.Contains(err.Error(), "broken pipe") {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":    "WebsocketTasklet",
				"registration": t.registration,
				"Address":      t.connection.RemoteAddr().String(),
				"id":           t.id,
				"Error":        err,
			}).Debugf("ping failed for websocket ant worker")
		}
		return err
	} else {
		t.registration.ReceivedAt = time.Now()
		return nil
	}
}

func (t *WebsocketTasklet) setupPingTicker(ctx context.Context) {
	t.ticker = time.NewTicker(time.Second * 5)
	go func() {
		for !t.isClosed() {
			select {
			case <-ctx.Done():
				t.ticker.Stop()
				return
			case <-t.ticker.C:
				if err := t.ping(); err != nil {
					t.Close(ctx)
					return
				}
			}
		}
	}()
}
