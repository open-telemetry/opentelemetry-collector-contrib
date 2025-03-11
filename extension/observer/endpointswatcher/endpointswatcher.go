// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package endpointswatcher // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"

import (
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ observer.Observable = (*EndpointsWatcher)(nil)

// EndpointsWatcher provides a generic mechanism to run EndpointsLister.ListEndpoints every
// RefreshInterval and report any new or removed endpoints to Notify instances registered
// via ListAndWatch. Any observer that lists endpoints can make use of EndpointsWatcher
// to poll for endpoints by embedding this struct and using NewEndpointsWatcher().
type EndpointsWatcher struct {
	EndpointsLister EndpointsLister
	RefreshInterval time.Duration

	// subscribed Notify instances ~sync.Map(map[NotifyID]Notify)
	toNotify sync.Map
	// map of NotifyID to known endpoints for that Notify (subscriptions can occur at different times in service startup).
	// ~sync.Map(map[NotifyID]map[EndpointID]Endpoint)
	existingEndpoints sync.Map
	stop              chan struct{}
	once              *sync.Once
	logger            *zap.Logger
}

func New(endpointsLister EndpointsLister, refreshInterval time.Duration, logger *zap.Logger) *EndpointsWatcher {
	return &EndpointsWatcher{
		EndpointsLister:   endpointsLister,
		RefreshInterval:   refreshInterval,
		existingEndpoints: sync.Map{},
		toNotify:          sync.Map{},
		stop:              make(chan struct{}),
		once:              &sync.Once{},
		logger:            logger,
	}
}

// ListAndWatch runs EndpointsLister.ListEndpoints() on a regular interval and keeps track of the results
// for alerting all subscribed Notify's of the based on the differences from the previous call.
func (ew *EndpointsWatcher) ListAndWatch(notify observer.Notify) {
	ew.once.Do(func() {
		go func() {
			ticker := time.NewTicker(ew.RefreshInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ew.stop:
					return
				case <-ticker.C:
					var toNotify []observer.NotifyID
					ew.toNotify.Range(func(notifyID, _ any) bool {
						toNotify = append(toNotify, notifyID.(observer.NotifyID))
						return true
					})
					ew.notifyOfLatestEndpoints(toNotify...)
				}
			}
		}()
	})

	ew.toNotify.Store(notify.ID(), notify)
	ew.notifyOfLatestEndpoints(notify.ID())
}

func (ew *EndpointsWatcher) Unsubscribe(notify observer.Notify) {
	ew.toNotify.Delete(notify.ID())
	ew.existingEndpoints.Delete(notify.ID())
}

// notifyOfLatestEndpoints alerts subscribed Notify instances by their NotifyID of latest Endpoint events,
// updating their internal store with results of ListEndpoints() call.
func (ew *EndpointsWatcher) notifyOfLatestEndpoints(notifyIDs ...observer.NotifyID) {
	latestEndpoints := ew.EndpointsLister.ListEndpoints()

	wg := &sync.WaitGroup{}
	for _, notifyID := range notifyIDs {
		var notify observer.Notify
		if n, ok := ew.toNotify.Load(notifyID); !ok {
			// an Unsubscribe() must have occurred during this call
			ew.logger.Debug("notifyOfEndpoints() ignoring instruction to notify non-subscribed Notify", zap.Any("notify", notifyID))
			continue
		} else if notify, ok = n.(observer.Notify); !ok {
			ew.logger.Warn("failed to obtain notify instance from EndpointsWatcher", zap.Any("notify", n))
			continue
		}
		wg.Add(1)
		go ew.updateAndNotifyOfEndpoints(notify, latestEndpoints, wg)
	}
	wg.Wait()
}

func (ew *EndpointsWatcher) updateAndNotifyOfEndpoints(notify observer.Notify, endpoints []observer.Endpoint, done *sync.WaitGroup) {
	defer done.Done()
	removedEndpoints, addedEndpoints, changedEndpoints := ew.updateEndpoints(notify, endpoints)
	if len(removedEndpoints) > 0 {
		ew.logEndpointEvent("removed endpoints", notify, removedEndpoints)
		notify.OnRemove(removedEndpoints)
	}

	if len(addedEndpoints) > 0 {
		ew.logEndpointEvent("added endpoints", notify, addedEndpoints)
		notify.OnAdd(addedEndpoints)
	}

	if len(changedEndpoints) > 0 {
		ew.logEndpointEvent("changed endpoints", notify, changedEndpoints)
		notify.OnChange(changedEndpoints)
	}
}

func (ew *EndpointsWatcher) updateEndpoints(notify observer.Notify, endpoints []observer.Endpoint) (removed, added, changed []observer.Endpoint) {
	notifyID := notify.ID()
	// Create map from ID to endpoint for lookup.
	endpointsMap := make(map[observer.EndpointID]struct{}, len(endpoints))
	for _, e := range endpoints {
		endpointsMap[e.ID] = struct{}{}
	}

	le, _ := ew.existingEndpoints.LoadOrStore(notifyID, map[observer.EndpointID]observer.Endpoint{})
	var storedEndpoints map[observer.EndpointID]observer.Endpoint
	var ok bool
	if storedEndpoints, ok = le.(map[observer.EndpointID]observer.Endpoint); !ok {
		ew.logger.Warn("failed to load Endpoint store from EndpointsWatcher", zap.Any("endpoints", le))
		return
	}
	// copy to not modify sync.Map value directly (will be reloaded)
	existingEndpoints := map[observer.EndpointID]observer.Endpoint{}
	for id, endpoint := range storedEndpoints {
		existingEndpoints[id] = endpoint
	}

	// Iterate over the latest endpoints obtained. An endpoint needs
	// to be added or updated in case it is not already available in existingEndpoints or doesn't match
	// the latest value.
	for _, e := range endpoints {
		var existingEndpoint observer.Endpoint
		if existingEndpoint, ok = existingEndpoints[e.ID]; !ok {
			existingEndpoints[e.ID] = e
			added = append(added, e)
		} else if !endpointsEqual(e, existingEndpoint) {
			// Collect updated endpoints.
			existingEndpoints[e.ID] = e
			changed = append(changed, e)
		}
	}

	// If endpoint present in existingEndpoints does not exist in the latest
	// list, it needs to be removed.
	for id, e := range existingEndpoints {
		if _, ok = endpointsMap[e.ID]; !ok {
			delete(existingEndpoints, id)
			removed = append(removed, e)
		}
	}

	ew.existingEndpoints.Store(notifyID, existingEndpoints)
	return
}

// StopListAndWatch polling the ListEndpoints.
func (ew *EndpointsWatcher) StopListAndWatch() {
	if ew.stop != nil {
		close(ew.stop)
	}
}

// EndpointsLister that provides a list of endpoints.
type EndpointsLister interface {
	// ListEndpoints provides a list of endpoints and is expected to be
	// implemented by an observer looking for endpoints.
	ListEndpoints() []observer.Endpoint
}

func (ew *EndpointsWatcher) logEndpointEvent(msg string, notify observer.Notify, endpoints []observer.Endpoint) {
	if ce := ew.logger.Check(zap.DebugLevel, msg); ce != nil {
		fields := []zap.Field{zap.Any("notify", notify.ID())}
		for _, endpoint := range endpoints {
			if env, err := endpoint.Env(); err == nil {
				if marshaled, e := json.Marshal(env); e == nil {
					fields = append(fields, zap.String(string(endpoint.ID), string(marshaled)))
				}
			}
		}
		ce.Write(fields...)
	}
}

func endpointsEqual(lhs, rhs observer.Endpoint) bool {
	switch {
	case lhs.ID != rhs.ID:
		return false
	case lhs.Target != rhs.Target:
		return false
	case lhs.Details == nil && rhs.Details != nil:
		return false
	case rhs.Details == nil && lhs.Details != nil:
		return false
	case lhs.Details == nil && rhs.Details == nil:
		return true
	case lhs.Details.Type() != rhs.Details.Type():
		return false
	default:
		return reflect.DeepEqual(lhs.Details.Env(), rhs.Details.Env())
	}
}
