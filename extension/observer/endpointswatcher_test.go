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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRefreshEndpointsOnStartup(t *testing.T) {
	lister, watcher, notify := setup(t)

	lister.addEndpoint(0)
	notify.numToBeAdded = 1
	watcher.ListAndWatch(notify)
	watcher.StopListAndWatch()

	// Endpoints available before the ListAndWatch call should be
	// readily discovered.
	expected := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))
}

func TestNotifyOfEndpoints(t *testing.T) {
	lister, watcher, notify := setup(t)
	defer watcher.StopListAndWatch()

	lister.addEndpoint(0)
	notify.numToBeAdded = 1
	watcher.ListAndWatch(notify)

	expected := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	lister.addEndpoint(1)
	lister.addEndpoint(2)
	notify.numToBeAdded = 2

	lister.removeEndpoint(0)
	notify.numToBeRemoved = 1

	watcher.notifyOfEndpoints(notify.ID())

	expected["1"] = Endpoint{ID: "1"}
	expected["2"] = Endpoint{ID: "2"}
	delete(expected, "0")
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	lister.updateEndpoint(2, "updated_target")
	notify.numToBeChanged = 1

	watcher.notifyOfEndpoints(notify.ID())

	expected["2"] = Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	watcher.Unsubscribe(notify)
	require.Nil(t, existingEndpoints(t, watcher, notify.ID()))
}

func TestNotifyOfEndpointsMultipleNotify(t *testing.T) {
	lister, watcher, notifyOne := setup(t)
	defer watcher.StopListAndWatch()

	lister.addEndpoint(0)
	notifyOne.numToBeAdded = 1
	watcher.ListAndWatch(notifyOne)

	notifyTwo := newMockNotififier(t, "notify2")
	lister.addEndpoint(1)
	notifyTwo.numToBeAdded = 2
	watcher.ListAndWatch(notifyTwo)

	expectedOne := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))

	expectedTwo := map[EndpointID]Endpoint{"0": {ID: "0"}, "1": {ID: "1"}}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	lister.addEndpoint(2)
	lister.addEndpoint(3)
	notifyOne.numToBeAdded = 3
	notifyTwo.numToBeAdded = 2
	lister.removeEndpoint(0)
	delete(expectedOne, "0")
	delete(expectedTwo, "0")
	notifyOne.numToBeRemoved = 1
	notifyTwo.numToBeRemoved = 1

	watcher.notifyOfEndpoints(notifyOne.ID(), notifyTwo.ID())

	expectedOne["1"] = Endpoint{ID: "1"}
	expectedOne["2"] = Endpoint{ID: "2"}
	expectedOne["3"] = Endpoint{ID: "3"}
	expectedTwo["2"] = Endpoint{ID: "2"}
	expectedTwo["3"] = Endpoint{ID: "3"}

	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	lister.updateEndpoint(2, "updated_target")
	notifyOne.numToBeChanged = 1
	notifyTwo.numToBeChanged = 1
	watcher.notifyOfEndpoints(notifyOne.ID(), notifyTwo.ID())

	expectedOne["2"] = Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))
	expectedTwo["2"] = Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	watcher.Unsubscribe(notifyOne)
	require.Nil(t, existingEndpoints(t, watcher, notifyOne.ID()))

	lister.addEndpoint(4)
	notifyTwo.numToBeAdded = 1
	watcher.ListAndWatch(notifyTwo)
	expectedTwo["4"] = Endpoint{ID: "4"}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	watcher.Unsubscribe(notifyTwo)
	require.Nil(t, existingEndpoints(t, watcher, notifyTwo.ID()))
}

func existingEndpoints(t *testing.T, watcher *EndpointsWatcher, id NotifyID) map[EndpointID]Endpoint {
	if existing, ok := watcher.existingEndpoints.Load(id); ok {
		endpoints, ok := existing.(map[EndpointID]Endpoint)
		assert.True(t, ok)
		return endpoints
	}
	return nil
}

func setup(t *testing.T) (*mockEndpointsLister, *EndpointsWatcher, *mockNotifier) {
	ml := &mockEndpointsLister{
		endpointsMap: map[EndpointID]Endpoint{},
	}

	ew := NewEndpointsWatcher(ml, 2*time.Second, zap.NewNop())
	mn := newMockNotififier(t, "mockNotifier")

	return ml, ew, mn
}

var _ Notify = (*mockNotifier)(nil)

type mockNotifier struct {
	t              *testing.T
	id             string
	numToBeAdded   int
	numToBeRemoved int
	numToBeChanged int
}

func newMockNotififier(t *testing.T, id string) *mockNotifier {
	return &mockNotifier{t: t, id: id}
}

func (m *mockNotifier) ID() NotifyID {
	return NotifyID(m.id)
}

func (m *mockNotifier) OnAdd(added []Endpoint) {
	require.Equal(m.t, m.numToBeAdded, len(added))
}

func (m *mockNotifier) OnRemove(removed []Endpoint) {
	require.Equal(m.t, m.numToBeRemoved, len(removed))
}

func (m *mockNotifier) OnChange(changed []Endpoint) {
	require.Equal(m.t, m.numToBeChanged, len(changed))
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
