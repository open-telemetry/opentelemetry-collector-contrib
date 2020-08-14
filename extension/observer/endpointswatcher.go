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
	"reflect"
	"time"
)

// EndpointsWatcher provides a generic mechanism to run ListEndpoints every
// RefreshInterval and report any new or removed endpoints using Notify
// passed into ListAndWatch. Any observer that lists endpoints can make
// use of EndpointsWatcher to poll for endpoints by embedding this struct
// in the observer struct.
type EndpointsWatcher struct {
	Endpointslister   EndpointsLister
	RefreshInterval   time.Duration
	existingEndpoints map[EndpointID]Endpoint
	stop              chan struct{}
}

// ListAndWatch runs ListEndpoints on a regular interval and keeps the list.
func (ew *EndpointsWatcher) ListAndWatch(listener Notify) {
	ew.existingEndpoints = map[EndpointID]Endpoint{}
	ew.stop = make(chan struct{})

	// Do the initial listing immediately so that services can be monitored ASAP.
	ew.refreshEndpoints(listener)

	go func() {
		ticker := time.NewTicker(ew.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ew.stop:
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
	latestEndpoints := ew.Endpointslister.ListEndpoints()

	// Create map from ID to endpoint for lookup.
	latestEndpointsMap := make(map[EndpointID]bool, len(latestEndpoints))
	for _, e := range latestEndpoints {
		latestEndpointsMap[e.ID] = true
	}

	var removedEndpoints, addedEndpoints, updatedEndpoints []Endpoint
	// Iterate over the latest endpoints obtained. An endpoint needs
	// to be added in case it is not already available in existingEndpoints.
	for _, e := range latestEndpoints {
		if existingEndpoint, ok := ew.existingEndpoints[e.ID]; !ok {
			ew.existingEndpoints[e.ID] = e
			addedEndpoints = append(addedEndpoints, e)
		} else {
			// Collect updated endpoints.
			if !reflect.DeepEqual(existingEndpoint, e) {
				ew.existingEndpoints[e.ID] = e
				updatedEndpoints = append(updatedEndpoints, e)
			}
		}
	}

	// If endpoint present in existingEndpoints does not exist in the latest
	// list, it needs to be removed.
	for id, e := range ew.existingEndpoints {
		if !latestEndpointsMap[e.ID] {
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

	if len(updatedEndpoints) > 0 {
		listener.OnChange(updatedEndpoints)
	}
}

// StopListAndWatch polling the ListEndpoints.
func (ew *EndpointsWatcher) StopListAndWatch() {
	close(ew.stop)
}

// EndpointsLister that provides a list of endpoints.
type EndpointsLister interface {
	// ListEndpoints provides a list of endpoints and is expected to be
	// implemented by an observer looking for endpoints.
	ListEndpoints() []Endpoint
}
