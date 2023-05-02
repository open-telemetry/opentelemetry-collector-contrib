// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package observer // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"

import (
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
)

var _ Observable = (*EndpointsWatcher)(nil)

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

func NewEndpointsWatcher(endpointsLister EndpointsLister, refreshInterval time.Duration, logger *zap.Logger) *EndpointsWatcher {
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
func (ew *EndpointsWatcher) ListAndWatch(notify Notify) {
	ew.once.Do(func() {
		go func() {
			ticker := time.NewTicker(ew.RefreshInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ew.stop:
					return
				case <-ticker.C:
					var toNotify []NotifyID
					ew.toNotify.Range(func(notifyID, _ interface{}) bool {
						toNotify = append(toNotify, notifyID.(NotifyID))
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

func (ew *EndpointsWatcher) Unsubscribe(notify Notify) {
	ew.toNotify.Delete(notify.ID())
	ew.existingEndpoints.Delete(notify.ID())
}

// notifyOfLatestEndpoints alerts subscribed Notify instances by their NotifyID of latest Endpoint events,
// updating their internal store with results of ListEndpoints() call.
func (ew *EndpointsWatcher) notifyOfLatestEndpoints(notifyIDs ...NotifyID) {
	latestEndpoints := ew.EndpointsLister.ListEndpoints()

	wg := &sync.WaitGroup{}
	for _, notifyID := range notifyIDs {
		var notify Notify
		if n, ok := ew.toNotify.Load(notifyID); !ok {
			// an Unsubscribe() must have occurred during this call
			ew.logger.Debug("notifyOfEndpoints() ignoring instruction to notify non-subscribed Notify", zap.Any("notify", notifyID))
			continue
		} else if notify, ok = n.(Notify); !ok {
			ew.logger.Warn("failed to obtain notify instance from EndpointsWatcher", zap.Any("notify", n))
			continue
		}
		wg.Add(1)
		go ew.updateAndNotifyOfEndpoints(notify, latestEndpoints, wg)
	}
	wg.Wait()
}

func (ew *EndpointsWatcher) updateAndNotifyOfEndpoints(notify Notify, endpoints []Endpoint, done *sync.WaitGroup) {
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

func (ew *EndpointsWatcher) updateEndpoints(notify Notify, endpoints []Endpoint) (removed, added, changed []Endpoint) {
	notifyID := notify.ID()
	// Create map from ID to endpoint for lookup.
	endpointsMap := make(map[EndpointID]struct{}, len(endpoints))
	for _, e := range endpoints {
		endpointsMap[e.ID] = struct{}{}
	}

	le, _ := ew.existingEndpoints.LoadOrStore(notifyID, map[EndpointID]Endpoint{})
	var storedEndpoints map[EndpointID]Endpoint
	var ok bool
	if storedEndpoints, ok = le.(map[EndpointID]Endpoint); !ok {
		ew.logger.Warn("failed to load Endpoint store from EndpointsWatcher", zap.Any("endpoints", le))
		return
	}
	// copy to not modify sync.Map value directly (will be reloaded)
	existingEndpoints := map[EndpointID]Endpoint{}
	for id, endpoint := range storedEndpoints {
		existingEndpoints[id] = endpoint
	}

	// Iterate over the latest endpoints obtained. An endpoint needs
	// to be added or updated in case it is not already available in existingEndpoints or doesn't match
	// the latest value.
	for _, e := range endpoints {
		var existingEndpoint Endpoint
		if existingEndpoint, ok = existingEndpoints[e.ID]; !ok {
			existingEndpoints[e.ID] = e
			added = append(added, e)
		} else if !existingEndpoint.equals(e) {
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
	ListEndpoints() []Endpoint
}

func (ew *EndpointsWatcher) logEndpointEvent(msg string, notify Notify, endpoints []Endpoint) {
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
