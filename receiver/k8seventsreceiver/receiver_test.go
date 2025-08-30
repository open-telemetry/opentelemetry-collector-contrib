// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector/k8sleaderelectortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/metadata"
)

func TestNewReceiver(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	rCfg.makeClient = func(k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewSimpleClientset(), nil
	}
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))

	rCfg.Namespaces = []string{"test", "another_test"}
	r1, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, r1)
	require.NoError(t, r1.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r1.Shutdown(t.Context()))
}

func TestHandleEvent(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	sink := new(consumertest.LogsSink)
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		sink,
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	recv := r.(*k8seventsReceiver)
	recv.ctx = t.Context()
	k8sEvent := getEvent()
	recv.handleEvent(k8sEvent)

	assert.Equal(t, 1, sink.LogRecordCount())
}

func TestDropEventsOlderThanStartupTime(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	sink := new(consumertest.LogsSink)
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		sink,
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	recv := r.(*k8seventsReceiver)
	recv.ctx = t.Context()
	k8sEvent := getEvent()
	k8sEvent.FirstTimestamp = v1.Time{Time: time.Now().Add(-time.Hour)}
	recv.handleEvent(k8sEvent)

	assert.Equal(t, 0, sink.LogRecordCount())
}

func TestGetEventTimestamp(t *testing.T) {
	k8sEvent := getEvent()
	eventTimestamp := getEventTimestamp(k8sEvent)
	assert.Equal(t, k8sEvent.FirstTimestamp.Time, eventTimestamp)

	k8sEvent.FirstTimestamp = v1.Time{Time: time.Now().Add(-time.Hour)}
	k8sEvent.LastTimestamp = v1.Now()
	eventTimestamp = getEventTimestamp(k8sEvent)
	assert.Equal(t, k8sEvent.LastTimestamp.Time, eventTimestamp)

	k8sEvent.FirstTimestamp = v1.Time{}
	k8sEvent.LastTimestamp = v1.Time{}
	k8sEvent.EventTime = v1.MicroTime(v1.Now())
	eventTimestamp = getEventTimestamp(k8sEvent)
	assert.Equal(t, k8sEvent.EventTime.Time, eventTimestamp)
}

func TestAllowEvent(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	recv := r.(*k8seventsReceiver)
	k8sEvent := getEvent()

	shouldAllowEvent := recv.allowEvent(k8sEvent)
	assert.True(t, shouldAllowEvent)

	k8sEvent.FirstTimestamp = v1.Time{Time: time.Now().Add(-time.Hour)}
	shouldAllowEvent = recv.allowEvent(k8sEvent)
	assert.False(t, shouldAllowEvent)

	k8sEvent.FirstTimestamp = v1.Time{}
	shouldAllowEvent = recv.allowEvent(k8sEvent)
	assert.False(t, shouldAllowEvent)
}

func TestReceiverWithLeaderElection(t *testing.T) {
	le := &k8sleaderelectortest.FakeLeaderElection{}
	host := &k8sleaderelectortest.FakeHost{FakeLeaderElection: le}
	leaderID := component.MustNewID("k8s_leader_elector")

	cfg := createDefaultConfig().(*Config)
	cfg.K8sLeaderElector = &leaderID
	cfg.makeClient = func(_ k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewSimpleClientset(), nil
	}

	sink := new(consumertest.LogsSink)
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		sink,
	)
	require.NoError(t, err)
	recv := r.(*k8seventsReceiver)

	require.NoError(t, r.Start(t.Context(), host))
	t.Cleanup(func() {
		assert.NoError(t, r.Shutdown(t.Context()))
	})

	// Become leader: start processing events
	le.InvokeOnLeading()
	recv.handleEvent(getEvent())

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 1
	}, 5*time.Second, 100*time.Millisecond, "logs not collected while leader")

	// lose leadership
	le.InvokeOnStopping()

	// DO NOT call recv.handleEvent(...) here; informer wouldn't deliver to this instance.
	// Give a tiny moment and ensure count stays 1.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, sink.LogRecordCount(), "event should be ignored after losing leadership")

	// regain leadership and inject again
	le.InvokeOnLeading()
	recv.handleEvent(getEvent())

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 2
	}, 5*time.Second, 100*time.Millisecond, "logs not collected after regaining leadership")
}

func getEvent() *corev1.Event {
	return &corev1.Event{
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       "test-34bcd-rn54",
			Namespace:  "test",
			UID:        types.UID("059f3edc-b5a9"),
		},
		Reason:         "testing_event_1",
		Count:          2,
		FirstTimestamp: v1.Now(),
		Type:           "Normal",
		Message:        "testing event message",
		ObjectMeta: v1.ObjectMeta{
			UID:               types.UID("289686f9-a5c0"),
			Name:              "1",
			Namespace:         "test",
			CreationTimestamp: v1.Now(),
		},
		Source: corev1.EventSource{
			Component: "testComponent",
			Host:      "testHost",
		},
	}
}
