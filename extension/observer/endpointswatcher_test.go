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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRefreshEndpointsOnStartup(t *testing.T) {
	endpointsMap = map[EndpointID]Endpoint{}

	ew := EndpointsWatcher{
		ListEndpoints:     listEndpoints,
		RefreshInterval:   2 * time.Second,
		existingEndpoints: map[EndpointID]Endpoint{},
	}

	mn := mockNotifier{}

	addEndpoint(0)
	ew.ListAndWatch(mn)
	ew.StopListAndWatch()

	// Endpoints available before the ListAndWatch call should be
	// readily discovered.
	expected := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, ew.existingEndpoints)
}

func TestRefreshEndpoints(t *testing.T) {
	endpointsMap = map[EndpointID]Endpoint{}

	ew := EndpointsWatcher{
		ListEndpoints:     listEndpoints,
		RefreshInterval:   2 * time.Second,
		existingEndpoints: map[EndpointID]Endpoint{},
	}

	mn := mockNotifier{}

	addEndpoint(0)
	ew.refreshEndpoints(mn)

	expected := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, ew.existingEndpoints)

	addEndpoint(1)
	addEndpoint(2)
	removeEndpoint(0)
	ew.refreshEndpoints(mn)

	expected["1"] = Endpoint{ID: "1"}
	expected["2"] = Endpoint{ID: "2"}
	delete(expected, "0")
	require.Equal(t, expected, ew.existingEndpoints)

	updateEndpoint(2, "updated_target")
	ew.refreshEndpoints(mn)

	expected["2"] = Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expected, ew.existingEndpoints)
}

var endpointsMap map[EndpointID]Endpoint

func addEndpoint(n int) {
	id := EndpointID(strconv.Itoa(n))
	e := Endpoint{ID: id}
	endpointsMap[id] = e
}

func removeEndpoint(n int) {
	id := EndpointID(strconv.Itoa(n))
	delete(endpointsMap, id)
}

func updateEndpoint(n int, target string) {
	id := EndpointID(strconv.Itoa(n))
	e := Endpoint{
		ID:     id,
		Target: target,
	}
	endpointsMap[id] = e
}

func listEndpoints() []Endpoint {
	endpoints := make([]Endpoint, 0)
	for _, e := range endpointsMap {
		endpoints = append(endpoints, e)
	}
	return endpoints
}

type mockNotifier struct {
}

var _ Notify = (*mockNotifier)(nil)

func (m mockNotifier) OnAdd([]Endpoint) {
}

func (m mockNotifier) OnRemove([]Endpoint) {
}

func (m mockNotifier) OnChange([]Endpoint) {
}
