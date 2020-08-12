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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRefreshEndpointsOnStartup(t *testing.T) {
	ml, ew, mn := setup()

	ml.addEndpoint(0)
	ew.ListAndWatch(mn)
	ew.StopListAndWatch()

	// Endpoints available before the ListAndWatch call should be
	// readily discovered.
	expected := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, ew.existingEndpoints)
}

func TestRefreshEndpoints(t *testing.T) {
	ml, ew, mn := setup()

	ml.addEndpoint(0)
	ew.refreshEndpoints(mn)

	expected := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, ew.existingEndpoints)

	ml.addEndpoint(1)
	ml.addEndpoint(2)
	ml.removeEndpoint(0)
	ew.refreshEndpoints(mn)

	expected["1"] = Endpoint{ID: "1"}
	expected["2"] = Endpoint{ID: "2"}
	delete(expected, "0")
	require.Equal(t, expected, ew.existingEndpoints)

	ml.updateEndpoint(2, "updated_target")
	ew.refreshEndpoints(mn)

	expected["2"] = Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expected, ew.existingEndpoints)
}

func setup() (*mockEndpointsLister, EndpointsWatcher, mockNotifier) {
	ml := &mockEndpointsLister{
		endpointsMap: map[EndpointID]Endpoint{},
	}

	ew := EndpointsWatcher{
		Endpointslister:   ml,
		RefreshInterval:   2 * time.Second,
		existingEndpoints: map[EndpointID]Endpoint{},
	}

	mn := mockNotifier{}

	return ml, ew, mn
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

type mockEndpointsLister struct {
	sync.Mutex
	endpointsMap map[EndpointID]Endpoint
}

func (m *mockEndpointsLister) addEndpoint(n int) {
	m.Lock()
	defer m.Unlock()

	id := EndpointID(strconv.Itoa(n))
	e := Endpoint{ID: id}
	m.endpointsMap[id] = e
}

func (m *mockEndpointsLister) removeEndpoint(n int) {
	m.Lock()
	defer m.Unlock()

	id := EndpointID(strconv.Itoa(n))
	delete(m.endpointsMap, id)
}

func (m *mockEndpointsLister) updateEndpoint(n int, target string) {
	m.Lock()
	defer m.Unlock()

	id := EndpointID(strconv.Itoa(n))
	e := Endpoint{
		ID:     id,
		Target: target,
	}
	m.endpointsMap[id] = e
}

func (m *mockEndpointsLister) ListEndpoints() []Endpoint {
	m.Lock()
	defer m.Unlock()

	out := make([]Endpoint, len(m.endpointsMap))

	i := 0
	for _, e := range m.endpointsMap {
		out[i] = e
		i++
	}

	return out
}

var _ EndpointsLister = (*mockEndpointsLister)(nil)
