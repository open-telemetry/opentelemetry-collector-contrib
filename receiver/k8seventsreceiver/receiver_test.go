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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sleaderelectortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/metadata"
)

func TestNewReceiver(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	rCfg.makeClient = func(k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewClientset(), nil
	}
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	rCfg.makeDynamicClient = func(k8sconfig.APIConfig) (dynamic.Interface, error) {
		return dynamicfake.NewSimpleDynamicClient(scheme), nil
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
	k8sEvent := getEvent("Normal")
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
	k8sEvent := getEvent("Normal")
	k8sEvent.FirstTimestamp = v1.Time{Time: time.Now().Add(-time.Hour)}
	recv.handleEvent(k8sEvent)

	assert.Equal(t, 0, sink.LogRecordCount())
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
	k8sEvent := getEvent("Normal")

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
		return fake.NewClientset(), nil
	}
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	cfg.makeDynamicClient = func(k8sconfig.APIConfig) (dynamic.Interface, error) {
		return dynamicfake.NewSimpleDynamicClient(scheme), nil
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

	// Become leader: start processing events
	le.InvokeOnLeading()
	recv.handleEvent(getEvent("Normal"))

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 1
	}, 5*time.Second, 100*time.Millisecond, "logs not collected while leader")

	// lose leadership - this will trigger Shutdown()
	le.InvokeOnStopping()

	// Verify count remains at 1 after losing leadership
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 1, sink.LogRecordCount(), "count should remain at 1 after losing leadership")

	// regain leadership
	le.InvokeOnLeading()
	recv.handleEvent(getEvent("Normal"))

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 2
	}, 5*time.Second, 100*time.Millisecond, "logs not collected after regaining leadership")

	// Final cleanup
	require.NoError(t, r.Shutdown(t.Context()))
}

func getEvent(eventType string) *corev1.Event {
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
		Type:           eventType,
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

// TestConfigGetK8sClient tests the getK8sClient method
func TestConfigGetK8sClient(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	cfg.makeClient = func(k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewClientset(), nil
	}

	client, err := cfg.getK8sClient()
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

// TestStartWithDynamicClientError tests Start when getDynamicClient fails
func TestStartWithDynamicClientError(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = func(k8sconfig.APIConfig) (dynamic.Interface, error) {
		return nil, assert.AnError
	}

	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)

	err = r.Start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err)
}

// TestStartWithUnknownLeaderElector tests Start when leader elector extension is not found
func TestStartWithUnknownLeaderElector(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	leaderID := component.MustNewID("unknown_elector")

	rCfg := createDefaultConfig().(*Config)
	rCfg.K8sLeaderElector = &leaderID
	rCfg.makeClient = func(k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewClientset(), nil
	}
	rCfg.makeDynamicClient = func(k8sconfig.APIConfig) (dynamic.Interface, error) {
		return dynamicfake.NewSimpleDynamicClient(scheme), nil
	}

	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)

	err = r.Start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown k8s leader elector")
}

// TestShutdownWithNilCancel tests Shutdown when cancel is nil
func TestShutdownWithNilCancel(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)

	recv := r.(*k8seventsReceiver)
	recv.cancel = nil

	err = recv.Shutdown(t.Context())
	assert.NoError(t, err)
}

// TestHandleEventWithConsumerError tests handleEvent when consumer returns an error
func TestHandleEventWithConsumerError(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)

	errConsumer := consumertest.NewErr(assert.AnError)

	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		errConsumer,
	)
	require.NoError(t, err)

	recv := r.(*k8seventsReceiver)
	recv.ctx = t.Context()

	k8sEvent := getEvent("Normal")
	recv.handleEvent(k8sEvent)
}

// TestAllowEventWithZeroTimestamp tests allowEvent with zero-value timestamp
func TestAllowEventWithZeroTimestamp(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)

	recv := r.(*k8seventsReceiver)

	k8sEvent := getEvent("Normal")
	k8sEvent.EventTime = v1.MicroTime{}
	k8sEvent.LastTimestamp = v1.Time{}
	k8sEvent.FirstTimestamp = v1.Time{}

	shouldAllowEvent := recv.allowEvent(k8sEvent)
	assert.False(t, shouldAllowEvent)
}

// TestStartWatchersMultipleNamespaces tests watching multiple namespaces
func TestStartWatchersMultipleNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme)

	rCfg := createDefaultConfig().(*Config)
	rCfg.Namespaces = []string{"default", "kube-system"}
	rCfg.makeClient = func(k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewClientset(), nil
	}
	rCfg.makeDynamicClient = func(k8sconfig.APIConfig) (dynamic.Interface, error) {
		return fakeClient, nil
	}

	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)

	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))

	recv := r.(*k8seventsReceiver)
	recv.mu.Lock()
	watcherCount := len(recv.stopperChanList)
	recv.mu.Unlock()

	// Should have at least 1 watcher started
	assert.GreaterOrEqual(t, watcherCount, 1)
	require.NoError(t, r.Shutdown(t.Context()))
}
