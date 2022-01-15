package wstask

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/resource"
	"sync"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				logrus.WithFields(logrus.Fields{
					"Component": "WebsocketTaskletUpgrader",
					"Origin":    origin,
					"Headers":   r.Header,
				}).Debugf("checking origin")
			}
			return true
		},
	}
)

// IOErrorHandler - for handling io errors
type IOErrorHandler func(id string)

// WSProxyRegistry for websockets
type WSProxyRegistry struct {
	id              string
	serverCfg       *config.ServerConfig
	resourceManager resource.Manager
	requestRegistry tasklet.RequestRegistry
	artifactManager *manager.ArtifactManager
	queueClient     queue.Client
	requestTopic    string
	connections     map[string]*WebsocketTasklet
	stopped         bool
	lock            sync.RWMutex
}

// NewWebsocketProxyRegistry creates new gateway for routing events to websocket clients
func NewWebsocketProxyRegistry(
	serverCfg *config.ServerConfig,
	resourceManager resource.Manager,
	requestRegistry tasklet.RequestRegistry,
	artifactManager *manager.ArtifactManager,
	queueClient queue.Client,
	requestTopic string,
	webserver web.Server) *WSProxyRegistry {
	registry := &WSProxyRegistry{
		id:              serverCfg.ID + "-websocket-proxy-registry",
		serverCfg:       serverCfg,
		resourceManager: resourceManager,
		requestRegistry: requestRegistry,
		artifactManager: artifactManager,
		queueClient:     queueClient,
		requestTopic:    requestTopic,
		connections:     make(map[string]*WebsocketTasklet),
	}
	webserver.GET("/ws/ants", registry.Register, acl.NewPermission(acl.Websocket, acl.Subscribe)).Name = "websocket_ants_register"
	return registry
}

// Start - startup routine
func (registry *WSProxyRegistry) Start(_ context.Context) (err error) {
	return nil
}

// Stop - stops websockets
func (registry *WSProxyRegistry) Stop(ctx context.Context) error {
	registry.lock.Lock()
	registry.stopped = true
	registry.lock.Unlock()
	for _, conn := range registry.connections {
		conn.Close(ctx)
	}
	return nil
}

// Register websocket subscription
func (registry *WSProxyRegistry) Register(c web.APIContext) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	ctx := context.Background()
	t, err := NewWebsocketTasklet(
		ctx,
		registry.serverCfg,
		registry.resourceManager,
		registry.requestRegistry,
		registry.queueClient,
		registry.requestTopic,
		ws,
		func(id string) {
			_ = registry.close(id)
		})
	if err != nil {
		return err
	}
	registry.lock.Lock()
	registry.connections[t.id] = t
	registry.lock.Unlock()
	return t.Start(ctx)
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (registry *WSProxyRegistry) close(id string) error {
	registry.lock.Lock()
	defer registry.lock.Unlock()
	websocketTasklet := registry.connections[id]
	if websocketTasklet == nil {
		return fmt.Errorf("no websocket websocketTasklet with id %s", id)
	}
	delete(registry.connections, id)
	websocketTasklet.Close(context.Background())
	return nil
}
