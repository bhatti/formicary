package gateway

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SubscriptionLease  keeps track of subscriptions
type SubscriptionLease struct {
	// EventType type of event
	EventType string `json:"event_type"`
	// EventScope scope of event
	EventScope string `json:"event_scope"`

	// private
	userID     string
	updatedAt  time.Time
	connection *websocket.Conn
	done       chan bool
	lock       *sync.RWMutex // this protects writes to websockets because only one goroutine can write at a time
}

// UnmarshalSubscriptionLease unmarshal
func UnmarshalSubscriptionLease(b []byte, conn *websocket.Conn) (lease *SubscriptionLease, err error) {
	lease = &SubscriptionLease{}
	if err := json.Unmarshal(b, lease); err != nil {
		return nil, err
	}
	lease.connection = conn
	lease.done = make(chan bool)

	if err := lease.Validate(); err != nil {
		return nil, err
	}
	return
}

// Validate validates subscription
func (lease *SubscriptionLease) Validate() error {
	if lease.EventType == "" {
		return fmt.Errorf("event-type is not specified")
	}
	if lease.connection == nil {
		return fmt.Errorf("connection is not specified")
	}
	return nil
}

// WriteMessage writes message
func (lease *SubscriptionLease) WriteMessage(payload []byte) error {
	lease.lock.Lock()
	defer lease.lock.Unlock()
	return lease.connection.WriteMessage(websocket.TextMessage, payload)
}

// Close closes connection
func (lease *SubscriptionLease) Close() {
	lease.lock.Lock()
	defer lease.lock.Unlock()
	defer func() {
		if r := recover(); r != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "SubscriptionLease",
				"Lease":     lease,
				"Recover":   r,
			}).Error("recovering from panic when closing channel")
		}
	}()
	close(lease.done)
	_ = lease.connection.Close()
}

// Key event key including address, userID and event type
func (lease *SubscriptionLease) Key() string {
	return EventKey(lease.Address(), lease.userID, lease.EventType, lease.EventScope)
}

// KeyWithoutAddress key including userID, event type and scope but no address
func (lease *SubscriptionLease) KeyWithoutAddress() string {
	return EventKey("", lease.userID, lease.EventType, lease.EventScope)
}

// Address remote address
func (lease *SubscriptionLease) Address() string {
	return lease.connection.RemoteAddr().String()
}

// EventKey event key including address, userID and event type
func EventKey(address string, userID string, eventType string, eventScope string) string {
	return fmt.Sprintf("%s::%s::%s::%s", address, userID, eventType, eventScope)
}
