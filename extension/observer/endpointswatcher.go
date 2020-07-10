// Copyright 2020, OpenTelemetry Authors
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

package observer

import (
	"time"
)

// EndpointsWatcher provides a generic mechanism to run ListEndpoints every
// RefreshInterval and report any new or removed endpoints using Notify
// passed into ListAndWatch. Any observer that lists endpoints can make
// use of EndpointsWatcher to poll for endpoints by embedding this struct
// in the observer struct.
type EndpointsWatcher struct {
	ListEndpoints     func() []Endpoint
	RefreshInterval   time.Duration
	existingEndpoints map[string]Endpoint
	stop              chan struct{}
}

// ListAndWatch runs ListEndpoints on a regular interval and keeps the list.
func (ew *EndpointsWatcher) ListAndWatch(listener Notify) {
	ew.existingEndpoints = make(map[string]Endpoint)
	ew.stop = make(chan struct{})

	ticker := time.NewTicker(ew.RefreshInterval)

	// Do the initial listing immediately so that services can be monitored ASAP.
	ew.refreshEndpoints(listener)

	go func() {
		for {
			select {
			case <-ew.stop:
				ticker.Stop()
				return
			case <-ticker.C:
				ew.refreshEndpoints(listener)
			}
		}
	}()
}

// refreshEndpoints updates the listener with the latest list
// of active endpoints.
func (ew *EndpointsWatcher) refreshEndpoints(listener Notify) {
	latestEndpoints := ew.ListEndpoints()

	// Create map from ID to endpoint for lookup.
	latestEndpointsMap := make(map[string]Endpoint, len(latestEndpoints))
	for _, e := range latestEndpoints {
		latestEndpointsMap[e.ID] = e
	}

	var removedEndpoints, addedEndpoints []Endpoint
	// Iterate over the latest endpoints obtained. An endpoint needs
	// to be added in case it is not already available in existingEndpoints.
	for _, e := range latestEndpoints {
		if _, ok := ew.existingEndpoints[e.ID]; !ok {
			ew.existingEndpoints[e.ID] = e
			addedEndpoints = append(addedEndpoints, e)
		}
	}

	// If endpoint present in existingEndpoints does not exist in the latest
	// list, it needs to be removed.
	for id, e := range ew.existingEndpoints {
		if _, ok := latestEndpointsMap[e.ID]; !ok {
			delete(ew.existingEndpoints, id)
			removedEndpoints = append(removedEndpoints, e)
		}
	}

	if len(removedEndpoints) > 0 {
		listener.OnRemove(removedEndpoints)
	}

	if len(addedEndpoints) > 0 {
		listener.OnAdd(addedEndpoints)
	}
}

// StopListAndWatch polling the ListEndpoints.
func (ew *EndpointsWatcher) StopListAndWatch() {
	close(ew.stop)
}
