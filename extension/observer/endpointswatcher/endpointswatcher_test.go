// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package endpointswatcher

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var (
	expectedEndpointZero  = observer.Endpoint{ID: observer.EndpointID(strconv.Itoa(0))}
	expectedEndpointOne   = observer.Endpoint{ID: observer.EndpointID(strconv.Itoa(1))}
	expectedEndpointTwo   = observer.Endpoint{ID: observer.EndpointID(strconv.Itoa(2))}
	expectedEndpointThree = observer.Endpoint{ID: observer.EndpointID(strconv.Itoa(3))}
	expectedEndpointFour  = observer.Endpoint{ID: observer.EndpointID(strconv.Itoa(4))}
)

func TestRefreshEndpointsOnStartup(t *testing.T) {
	lister, watcher, notify := setup(t)

	lister.addEndpoint(0)
	notify.On("OnAdd", []observer.Endpoint{expectedEndpointZero})
	watcher.ListAndWatch(notify)
	watcher.StopListAndWatch()

	notify.AssertExpectations(t)

	// Endpoints available before the ListAndWatch call should be
	// readily discovered.
	expected := map[observer.EndpointID]observer.Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))
}

func TestNotifyOfLatestEndpoints(t *testing.T) {
	lister, watcher, notify := setup(t)
	defer watcher.StopListAndWatch()

	zeroAdded := notify.On("OnAdd", []observer.Endpoint{expectedEndpointZero})
	lister.addEndpoint(0)
	watcher.ListAndWatch(notify)
	notify.AssertExpectations(t)
	zeroAdded.Unset()

	expected := map[observer.EndpointID]observer.Endpoint{"0": {ID: "0"}}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	oneAndTwoAdded := notify.On("OnAdd", []observer.Endpoint{expectedEndpointOne, expectedEndpointTwo})
	lister.addEndpoint(1)
	lister.addEndpoint(2)

	zeroRemoved := notify.On("OnRemove", []observer.Endpoint{expectedEndpointZero})
	lister.removeEndpoint(0)

	watcher.notifyOfLatestEndpoints(notify.ID())
	notify.AssertExpectations(t)
	oneAndTwoAdded.Unset()
	zeroRemoved.Unset()

	expected["1"] = observer.Endpoint{ID: "1"}
	expected["2"] = observer.Endpoint{ID: "2"}
	delete(expected, "0")
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	expectedUpdatedTwo := expectedEndpointTwo
	expectedUpdatedTwo.Target = "updated_target"
	twoChanged := notify.On("OnChange", []observer.Endpoint{expectedUpdatedTwo})
	lister.updateEndpoint(2, "updated_target")

	watcher.notifyOfLatestEndpoints(notify.ID())
	notify.AssertExpectations(t)
	twoChanged.Unset()

	expected["2"] = observer.Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expected, existingEndpoints(t, watcher, notify.ID()))

	watcher.Unsubscribe(notify)
	require.Nil(t, existingEndpoints(t, watcher, notify.ID()))
}

func TestNotifyOfLatestEndpointsMultipleNotify(t *testing.T) {
	lister, watcher, notifyOne := setup(t)
	defer watcher.StopListAndWatch()

	lister.addEndpoint(0)
	zeroAdded := notifyOne.On("OnAdd", []observer.Endpoint{expectedEndpointZero})
	watcher.ListAndWatch(notifyOne)
	notifyOne.AssertExpectations(t)
	zeroAdded.Unset()

	notifyTwo := &mockNotifier{id: "notify2"}
	lister.addEndpoint(1)
	zeroAndOneAdded := notifyTwo.On("OnAdd", []observer.Endpoint{expectedEndpointZero, expectedEndpointOne})
	watcher.ListAndWatch(notifyTwo)
	notifyTwo.AssertExpectations(t)
	zeroAndOneAdded.Unset()

	expectedOne := map[observer.EndpointID]observer.Endpoint{"0": {ID: "0"}}
	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))
	expectedTwo := map[observer.EndpointID]observer.Endpoint{"0": {ID: "0"}, "1": {ID: "1"}}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	lister.addEndpoint(2)
	lister.addEndpoint(3)
	oneTwoAndThreeAdded := notifyOne.On("OnAdd", []observer.Endpoint{expectedEndpointOne, expectedEndpointTwo, expectedEndpointThree})
	twoAndThreeAdded := notifyTwo.On("OnAdd", []observer.Endpoint{expectedEndpointTwo, expectedEndpointThree})
	lister.removeEndpoint(0)
	notifyOne.On("OnRemove", []observer.Endpoint{expectedEndpointZero})
	notifyTwo.On("OnRemove", []observer.Endpoint{expectedEndpointZero})
	watcher.notifyOfLatestEndpoints(notifyOne.ID(), notifyTwo.ID())
	notifyOne.AssertExpectations(t)
	notifyTwo.AssertExpectations(t)
	oneTwoAndThreeAdded.Unset()
	twoAndThreeAdded.Unset()

	delete(expectedOne, "0")
	expectedOne["1"] = observer.Endpoint{ID: "1"}
	expectedOne["2"] = observer.Endpoint{ID: "2"}
	expectedOne["3"] = observer.Endpoint{ID: "3"}
	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))

	delete(expectedTwo, "0")
	expectedTwo["2"] = observer.Endpoint{ID: "2"}
	expectedTwo["3"] = observer.Endpoint{ID: "3"}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	expectedUpdatedTwo := expectedEndpointTwo
	expectedUpdatedTwo.Target = "updated_target"
	oneTwoUpdated := notifyOne.On("OnChange", []observer.Endpoint{expectedUpdatedTwo})
	twoTwoUpdated := notifyTwo.On("OnChange", []observer.Endpoint{expectedUpdatedTwo})
	lister.updateEndpoint(2, "updated_target")
	watcher.notifyOfLatestEndpoints(notifyOne.ID(), notifyTwo.ID())
	notifyOne.AssertExpectations(t)
	notifyTwo.AssertExpectations(t)
	oneTwoUpdated.Unset()
	twoTwoUpdated.Unset()

	expectedOne["2"] = observer.Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expectedOne, existingEndpoints(t, watcher, notifyOne.ID()))
	expectedTwo["2"] = observer.Endpoint{ID: "2", Target: "updated_target"}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	watcher.Unsubscribe(notifyOne)
	require.Nil(t, existingEndpoints(t, watcher, notifyOne.ID()))

	lister.addEndpoint(4)
	fourAdded := notifyTwo.On("OnAdd", []observer.Endpoint{expectedEndpointFour})
	watcher.ListAndWatch(notifyTwo)
	notifyOne.AssertExpectations(t)
	notifyTwo.AssertExpectations(t)
	fourAdded.Unset()

	expectedTwo["4"] = observer.Endpoint{ID: "4"}
	require.Equal(t, expectedTwo, existingEndpoints(t, watcher, notifyTwo.ID()))

	watcher.Unsubscribe(notifyTwo)
	require.Nil(t, existingEndpoints(t, watcher, notifyTwo.ID()))
}

func existingEndpoints(tb testing.TB, watcher *EndpointsWatcher, id observer.NotifyID) map[observer.EndpointID]observer.Endpoint {
	if existing, ok := watcher.existingEndpoints.Load(id); ok {
		endpoints, ok := existing.(map[observer.EndpointID]observer.Endpoint)
		assert.True(tb, ok)
		return endpoints
	}
	return nil
}

func setup(tb testing.TB) (*mockEndpointsLister, *EndpointsWatcher, *mockNotifier) {
	ml := &mockEndpointsLister{
		endpointsMap: map[observer.EndpointID]observer.Endpoint{},
	}

	ew := New(ml, 2*time.Second, zaptest.NewLogger(tb))
	mn := &mockNotifier{id: "mockNotifier"}

	return ml, ew, mn
}

var _ observer.Notify = (*mockNotifier)(nil)

type mockNotifier struct {
	mock.Mock
	id string
}

func (m *mockNotifier) ID() observer.NotifyID {
	return observer.NotifyID(m.id)
}

func (m *mockNotifier) OnAdd(added []observer.Endpoint) {
	m.Called(sortEndpoints(added))
}

func (m *mockNotifier) OnRemove(removed []observer.Endpoint) {
	m.Called(sortEndpoints(removed))
}

func (m *mockNotifier) OnChange(changed []observer.Endpoint) {
	m.Called(sortEndpoints(changed))
}

func sortEndpoints(endpoints []observer.Endpoint) []observer.Endpoint {
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].ID < endpoints[j].ID })
	return endpoints
}

type mockEndpointsLister struct {
	sync.Mutex
	endpointsMap map[observer.EndpointID]observer.Endpoint
}

func (m *mockEndpointsLister) addEndpoint(n int) {
	m.Lock()
	defer m.Unlock()

	id := observer.EndpointID(strconv.Itoa(n))
	e := observer.Endpoint{ID: id}
	m.endpointsMap[id] = e
}

func (m *mockEndpointsLister) removeEndpoint(n int) {
	m.Lock()
	defer m.Unlock()

	id := observer.EndpointID(strconv.Itoa(n))
	delete(m.endpointsMap, id)
}

func (m *mockEndpointsLister) updateEndpoint(n int, target string) {
	m.Lock()
	defer m.Unlock()

	id := observer.EndpointID(strconv.Itoa(n))
	e := observer.Endpoint{
		ID:     id,
		Target: target,
	}
	m.endpointsMap[id] = e
}

func (m *mockEndpointsLister) ListEndpoints() []observer.Endpoint {
	m.Lock()
	defer m.Unlock()

	out := make([]observer.Endpoint, len(m.endpointsMap))

	i := 0
	for _, e := range m.endpointsMap {
		out[i] = e
		i++
	}

	return out
}

var _ EndpointsLister = (*mockEndpointsLister)(nil)

func TestEndpointsEqual(t *testing.T) {
	tests := []struct {
		name     string
		first    observer.Endpoint
		second   observer.Endpoint
		areEqual bool
	}{
		{
			name:  "equal empty endpoints",
			first: observer.Endpoint{}, second: observer.Endpoint{},
			areEqual: true,
		},
		{
			name:     "equal ID",
			first:    observer.Endpoint{ID: "id"},
			second:   observer.Endpoint{ID: "id"},
			areEqual: true,
		},
		{
			name:     "unequal ID",
			first:    observer.Endpoint{ID: "first"},
			second:   observer.Endpoint{ID: "second"},
			areEqual: false,
		},
		{
			name:     "equal Target",
			first:    observer.Endpoint{Target: "target"},
			second:   observer.Endpoint{Target: "target"},
			areEqual: true,
		},
		{
			name:     "unequal Target",
			first:    observer.Endpoint{Target: "first"},
			second:   observer.Endpoint{Target: "second"},
			areEqual: false,
		},
		{
			name:     "equal empty observer.Port",
			first:    observer.Endpoint{Details: &observer.Port{}},
			second:   observer.Endpoint{Details: &observer.Port{}},
			areEqual: true,
		},
		{
			name:     "equal observer.Port Name",
			first:    observer.Endpoint{Details: &observer.Port{Name: "port_name"}},
			second:   observer.Endpoint{Details: &observer.Port{Name: "port_name"}},
			areEqual: true,
		},
		{
			name:     "unequal observer.Port Name",
			first:    observer.Endpoint{Details: &observer.Port{Name: "first"}},
			second:   observer.Endpoint{Details: &observer.Port{Name: "second"}},
			areEqual: false,
		},
		{
			name:     "equal observer.Port observer.Port",
			first:    observer.Endpoint{Details: &observer.Port{Port: 2379}},
			second:   observer.Endpoint{Details: &observer.Port{Port: 2379}},
			areEqual: true,
		},
		{
			name:     "unequal observer.Port observer.Port",
			first:    observer.Endpoint{Details: &observer.Port{Port: 0}},
			second:   observer.Endpoint{Details: &observer.Port{Port: 1}},
			areEqual: false,
		},
		{
			name:     "equal observer.Port Transport",
			first:    observer.Endpoint{Details: &observer.Port{Transport: "transport"}},
			second:   observer.Endpoint{Details: &observer.Port{Transport: "transport"}},
			areEqual: true,
		},
		{
			name:     "unequal observer.Port Transport",
			first:    observer.Endpoint{Details: &observer.Port{Transport: "first"}},
			second:   observer.Endpoint{Details: &observer.Port{Transport: "second"}},
			areEqual: false,
		},
		{
			name: "equal observer.Port",
			first: observer.Endpoint{
				ID:     observer.EndpointID("port_id"),
				Target: "192.68.73.2",
				Details: &observer.Port{
					Name: "port_name",
					Pod: observer.Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"label_key": "label_val",
						},
						Annotations: map[string]string{
							"annotation_1": "value_1",
						},
						Namespace: "pod-namespace",
						UID:       "pod-uid",
					},
					Port:      2379,
					Transport: observer.ProtocolTCP,
				},
			},
			second: observer.Endpoint{
				ID:     observer.EndpointID("port_id"),
				Target: "192.68.73.2",
				Details: &observer.Port{
					Name: "port_name",
					Pod: observer.Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"label_key": "label_val",
						},
						Annotations: map[string]string{
							"annotation_1": "value_1",
						},
						Namespace: "pod-namespace",
						UID:       "pod-uid",
					},
					Port:      2379,
					Transport: observer.ProtocolTCP,
				},
			},
			areEqual: true,
		},
		{
			name: "unequal observer.Port Pod Label",
			first: observer.Endpoint{
				ID:     observer.EndpointID("port_id"),
				Target: "192.68.73.2",
				Details: &observer.Port{
					Name: "port_name",
					Pod: observer.Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"key_one": "val_one",
						},
						Annotations: map[string]string{
							"annotation_1": "value_1",
						},
						Namespace: "pod-namespace",
						UID:       "pod-uid",
					},
					Port:      2379,
					Transport: observer.ProtocolTCP,
				},
			},
			second: observer.Endpoint{
				ID:     observer.EndpointID("port_id"),
				Target: "192.68.73.2",
				Details: &observer.Port{
					Name: "port_name",
					Pod: observer.Pod{
						Name: "pod_name",
						Labels: map[string]string{
							"key_two": "val_two",
						},
						Annotations: map[string]string{
							"annotation_1": "value_1",
						},
						Namespace: "pod-namespace",
						UID:       "pod-uid",
					},
					Port:      2379,
					Transport: observer.ProtocolTCP,
				},
			},
			areEqual: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, endpointsEqual(tt.first, tt.second), tt.areEqual)
			require.Equal(t, endpointsEqual(tt.second, tt.first), tt.areEqual)
		})
	}
}
