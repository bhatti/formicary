package gateway

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// LeaseRegistry keeps track of subscriptions
type LeaseRegistry struct {
	leasesByKey            map[string]*SubscriptionLease
	keysByUserAndEventType map[string]map[string]bool
	keysByAddresses        map[string]map[string]bool
	locksByAddresses       map[string]*sync.RWMutex
	lock                   sync.RWMutex
}

// NewLeaseRegistry constructor
func NewLeaseRegistry() *LeaseRegistry {
	return &LeaseRegistry{
		leasesByKey:            make(map[string]*SubscriptionLease),
		keysByUserAndEventType: make(map[string]map[string]bool),
		keysByAddresses:        make(map[string]map[string]bool),
		locksByAddresses:       make(map[string]*sync.RWMutex),
	}
}

// Add - adds subscription lease
func (r *LeaseRegistry) Add(lease *SubscriptionLease) (err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if lease == nil {
		return fmt.Errorf("lease not specified")
	}
	if err = lease.Validate(); err != nil {
		return err
	}

	leaseLock := r.locksByAddresses[lease.Address()]
	if leaseLock == nil {
		leaseLock = &sync.RWMutex{}
		r.locksByAddresses[lease.Address()] = leaseLock
	}

	old := r.leasesByKey[lease.Key()]
	if old != nil {
		old.updatedAt = time.Now()
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":   "LeaseRegistry",
				"UserID":      lease.userID,
				"StateChange": lease.EventType,
				"Address":     lease.Address(),
				"Key":         lease.Key(),
			}).Debugf("updated lease")
		}
	} else {
		lease.updatedAt = time.Now()
		lease.lock = leaseLock
		r.leasesByKey[lease.Key()] = lease

		// Storing keys by userID and event-type so that we can publish events to just right users
		keys := r.keysByUserAndEventType[lease.KeyWithoutAddress()]
		if keys == nil {
			keys = map[string]bool{}
		}
		keys[lease.Key()] = true
		r.keysByUserAndEventType[lease.KeyWithoutAddress()] = keys

		// Storing keys by address so that we can unsubscribe all subscriptions when connection dies
		keys = r.keysByAddresses[lease.Address()]
		if keys == nil {
			keys = map[string]bool{}
		}
		keys[lease.Key()] = true
		r.keysByAddresses[lease.Address()] = keys

		logrus.WithFields(logrus.Fields{
			"Component":  "LeaseRegistry",
			"UserID":     lease.userID,
			"EventType":  lease.EventType,
			"EventScope": lease.EventScope,
			"Address":    lease.Address(),
			"Key":        lease.Key(),
		}).Infof("added lease")
	}
	return
}

// Remove - removes subscription lease
func (r *LeaseRegistry) Remove(lease *SubscriptionLease) (err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if lease == nil {
		return fmt.Errorf("lease not specified")
	}
	if err = lease.Validate(); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"Component":   "LeaseRegistry",
		"UserID":      lease.userID,
		"StateChange": lease.EventType,
		"Key":         lease.Key(),
	}).Info("removing lease!")

	lease.Close()
	delete(r.leasesByKey, lease.Key())

	// removing mapping of userID-event-type to lease key
	keys := r.keysByUserAndEventType[lease.KeyWithoutAddress()]
	if keys != nil {
		delete(keys, lease.Key())
		if len(keys) == 0 {
			delete(r.keysByUserAndEventType, lease.KeyWithoutAddress())
		} else {
			r.keysByUserAndEventType[lease.KeyWithoutAddress()] = keys
		}
	}

	// removing mapping of remote address to lease key
	keys = r.keysByAddresses[lease.Address()]
	if keys != nil {
		delete(keys, lease.Key())
		if len(keys) == 0 {
			delete(r.keysByAddresses, lease.Address())
			delete(r.locksByAddresses, lease.Address())
		} else {
			r.keysByAddresses[lease.Address()] = keys
		}
	}
	return
}

// Notify - notifies event
func (r *LeaseRegistry) Notify(userID string, eventType string, eventScope string, payload []byte) {
	// notify asynchronously
	leases := r.getLeasesByUserAndEventTypeScope(userID, eventType, eventScope)

	if len(leases) == 0 {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":   "LeaseRegistry",
				"StateChange": eventType,
				"UserID":      userID,
				"Event":       eventType,
				"Scope":       eventScope,
			}).Debugf("no leases found")
		}
		return
	}
	go func() {
		success := 0
		errors := 0
		for _, lease := range leases {
			if err := lease.WriteMessage(payload); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":   "LeaseRegistry",
					"UserID":      lease.userID,
					"StateChange": eventType,
					"Key":         lease.Key(),
					"Error":       err,
				}).Warnf("failed to send websocket event")
				errors++
			} else {
				success++
			}
		}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":   "LeaseRegistry",
				"UserID":      userID,
				"StateChange": eventType,
				"Success":     success,
				"Errors":      errors,
				"Leases":      len(leases),
			}).Debugf("Notification from Websocket Gateway")
		}
	}()
}

// getAllLeases
func (r *LeaseRegistry) getAllLeases() (leases []*SubscriptionLease) {
	r.lock.Lock()
	defer r.lock.Unlock()
	leases = make([]*SubscriptionLease, 0)
	for _, lease := range r.leasesByKey {
		leases = append(leases, lease)
	}
	return
}

// getLeasesByAddress
func (r *LeaseRegistry) getLeasesByAddress(address string) (leases []*SubscriptionLease) {
	r.lock.Lock()
	defer r.lock.Unlock()
	leases = make([]*SubscriptionLease, 0)
	keys := r.keysByAddresses[address]
	if keys != nil {
		dirty := false
		for key := range keys {
			lease := r.leasesByKey[key]
			if lease != nil {
				leases = append(leases, lease)
			} else {
				delete(keys, key)
				dirty = true
			}
		}
		if dirty {
			r.keysByAddresses[address] = keys
		}
	}
	return
}

// getLeasesByUserAndEventTypeScope accessor
func (r *LeaseRegistry) getLeasesByUserAndEventTypeScope(userID string, eventType string, eventScope string) (leases []*SubscriptionLease) {
	eventKey := EventKey("", userID, eventType, eventScope)
	r.lock.Lock()
	defer r.lock.Unlock()
	leases = make([]*SubscriptionLease, 0)
	keys := r.keysByUserAndEventType[eventKey]
	if keys != nil {
		dirty := false
		for key := range keys {
			lease := r.leasesByKey[key]
			if lease != nil {
				leases = append(leases, lease)
			} else {
				delete(keys, key)
				dirty = true
			}
		}
		if dirty {
			r.keysByUserAndEventType[eventKey] = keys
		}
	}
	return
}
