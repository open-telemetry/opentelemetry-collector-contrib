// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observer

import (
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	expectedEndpointZero  = Endpoint{ID: EndpointID(strconv.Itoa(0))}
	expectedEndpointOne   = Endpoint{ID: EndpointID(strconv.Itoa(1))}
	expectedEndpointTwo   = Endpoint{ID: EndpointID(strconv.Itoa(2))}
	expectedEndpointThree = Endpoint{ID: EndpointID(strconv.Itoa(3))}
	expectedEndpointFour  = Endpoint{ID: EndpointID(strconv.Itoa(4))}
)

func TestRefreshEndpointsOnStartup(t *testing.T) {
	lister, watcher, notify := setup(t)

	lister.addEndpoint(0)
	notify.On("OnAdd", []Endpoint{expectedEndpointZero})
	watcher.ListAndWatch(notify)
	watcher.StopListAndWatch()

	notify.AssertExpectations(t)

	// Endpoints available before the ListAndWatch call should be
	// readily discovered.
	expected := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))
}

func TestNotifyOfLatestEndpoints(t *testing.T) {
	lister, watcher, notify := setup(t)
	defer watcher.StopListAndWatch()

	zeroAdded := notify.On("OnAdd", []Endpoint{expectedEndpointZero})
	lister.addEndpoint(0)
	watcher.ListAndWatch(notify)
	notify.AssertExpectations(t)
	zeroAdded.Unset()

	expected := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	oneAndTwoAdded := notify.On("OnAdd", []Endpoint{expectedEndpointOne, expectedEndpointTwo})
	lister.addEndpoint(1)
	lister.addEndpoint(2)

	zeroRemoved := notify.On("OnRemove", []Endpoint{expectedEndpointZero})
	lister.removeEndpoint(0)

	watcher.notifyOfLatestEndpoints(notify.ID())
	notify.AssertExpectations(t)
	oneAndTwoAdded.Unset()
	zeroRemoved.Unset()

	expected["1"] = Endpoint{ID: "1"}
	expected["2"] = Endpoint{ID: "2"}
	delete(expected, "0")
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	expectedUpdatedTwo := expectedEndpointTwo
	expectedUpdatedTwo.Target = "updated_target"
	twoChanged := notify.On("OnChange", []Endpoint{expectedUpdatedTwo})
	lister.updateEndpoint(2, "updated_target")

	watcher.notifyOfLatestEndpoints(notify.ID())
	notify.AssertExpectations(t)
	twoChanged.Unset()

	expected["2"] = Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	watcher.Unsubscribe(notify)
	require.Nil(t, existingEndpoints(t, watcher, notify.ID()))
}

func TestNotifyOfLatestEndpointsMultipleNotify(t *testing.T) {
	lister, watcher, notifyOne := setup(t)
	defer watcher.StopListAndWatch()

	lister.addEndpoint(0)
	zeroAdded := notifyOne.On("OnAdd", []Endpoint{expectedEndpointZero})
	watcher.ListAndWatch(notifyOne)
	notifyOne.AssertExpectations(t)
	zeroAdded.Unset()

	notifyTwo := &mockNotifier{id: "notify2"}
	lister.addEndpoint(1)
	zeroAndOneAdded := notifyTwo.On("OnAdd", []Endpoint{expectedEndpointZero, expectedEndpointOne})
	watcher.ListAndWatch(notifyTwo)
	notifyTwo.AssertExpectations(t)
	zeroAndOneAdded.Unset()

	expectedOne := map[EndpointID]Endpoint{"0": {ID: "0"}}
	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))
	expectedTwo := map[EndpointID]Endpoint{"0": {ID: "0"}, "1": {ID: "1"}}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	lister.addEndpoint(2)
	lister.addEndpoint(3)
	oneTwoAndThreeAdded := notifyOne.On("OnAdd", []Endpoint{expectedEndpointOne, expectedEndpointTwo, expectedEndpointThree})
	twoAndThreeAdded := notifyTwo.On("OnAdd", []Endpoint{expectedEndpointTwo, expectedEndpointThree})
	lister.removeEndpoint(0)
	notifyOne.On("OnRemove", []Endpoint{expectedEndpointZero})
	notifyTwo.On("OnRemove", []Endpoint{expectedEndpointZero})
	watcher.notifyOfLatestEndpoints(notifyOne.ID(), notifyTwo.ID())
	notifyOne.AssertExpectations(t)
	notifyTwo.AssertExpectations(t)
	oneTwoAndThreeAdded.Unset()
	twoAndThreeAdded.Unset()

	delete(expectedOne, "0")
	expectedOne["1"] = Endpoint{ID: "1"}
	expectedOne["2"] = Endpoint{ID: "2"}
	expectedOne["3"] = Endpoint{ID: "3"}
	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))

	delete(expectedTwo, "0")
	expectedTwo["2"] = Endpoint{ID: "2"}
	expectedTwo["3"] = Endpoint{ID: "3"}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	expectedUpdatedTwo := expectedEndpointTwo
	expectedUpdatedTwo.Target = "updated_target"
	oneTwoUpdated := notifyOne.On("OnChange", []Endpoint{expectedUpdatedTwo})
	twoTwoUpdated := notifyTwo.On("OnChange", []Endpoint{expectedUpdatedTwo})
	lister.updateEndpoint(2, "updated_target")
	watcher.notifyOfLatestEndpoints(notifyOne.ID(), notifyTwo.ID())
	notifyOne.AssertExpectations(t)
	notifyTwo.AssertExpectations(t)
	oneTwoUpdated.Unset()
	twoTwoUpdated.Unset()

	expectedOne["2"] = Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))
	expectedTwo["2"] = Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	watcher.Unsubscribe(notifyOne)
	require.Nil(t, existingEndpoints(t, watcher, notifyOne.ID()))

	lister.addEndpoint(4)
	fourAdded := notifyTwo.On("OnAdd", []Endpoint{expectedEndpointFour})
	watcher.ListAndWatch(notifyTwo)
	notifyOne.AssertExpectations(t)
	notifyTwo.AssertExpectations(t)
	fourAdded.Unset()

	expectedTwo["4"] = Endpoint{ID: "4"}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	watcher.Unsubscribe(notifyTwo)
	require.Nil(t, existingEndpoints(t, watcher, notifyTwo.ID()))
}

func existingEndpoints(t testing.TB, watcher *EndpointsWatcher, id NotifyID) map[EndpointID]Endpoint {
	if existing, ok := watcher.existingEndpoints.Load(id); ok {
		endpoints, ok := existing.(map[EndpointID]Endpoint)
		assert.True(t, ok)
		return endpoints
	}
	return nil
}

func setup(t testing.TB) (*mockEndpointsLister, *EndpointsWatcher, *mockNotifier) {
	ml := &mockEndpointsLister{
		endpointsMap: map[EndpointID]Endpoint{},
	}

	ew := NewEndpointsWatcher(ml, 2*time.Second, zaptest.NewLogger(t))
	mn := &mockNotifier{id: "mockNotifier"}

	return ml, ew, mn
}

var _ Notify = (*mockNotifier)(nil)

type mockNotifier struct {
	mock.Mock
	id string
}

func (m *mockNotifier) ID() NotifyID {
	return NotifyID(m.id)
}

func (m *mockNotifier) OnAdd(added []Endpoint) {
	m.Called(sortEndpoints(added))
}

func (m *mockNotifier) OnRemove(removed []Endpoint) {
	m.Called(sortEndpoints(removed))
}

func (m *mockNotifier) OnChange(changed []Endpoint) {
	m.Called(sortEndpoints(changed))
}

func sortEndpoints(endpoints []Endpoint) []Endpoint {
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].ID < endpoints[j].ID })
	return endpoints
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
