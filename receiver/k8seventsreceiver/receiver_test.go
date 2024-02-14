// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestNewReceiver(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	rCfg.makeClient = func(apiConf k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewSimpleClientset(), nil
	}
	r, err := newReceiver(
		receivertest.NewNopCreateSettings(),
		rCfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(context.Background()))

	rCfg.Namespaces = []string{"test", "another_test"}
	r1, err := newReceiver(
		receivertest.NewNopCreateSettings(),
		rCfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, r1)
	require.NoError(t, r1.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, r1.Shutdown(context.Background()))
}

func TestHandleEvent(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	sink := new(consumertest.LogsSink)
	r, err := newReceiver(
		receivertest.NewNopCreateSettings(),
		rCfg,
		sink,
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	recv := r.(*k8seventsReceiver)
	recv.ctx = context.Background()
	k8sEvent := getEvent()
	recv.handleEvent(k8sEvent)

	assert.Equal(t, sink.LogRecordCount(), 1)
}

func TestDropEventsOlderThanStartupTime(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)
	sink := new(consumertest.LogsSink)
	r, err := newReceiver(
		receivertest.NewNopCreateSettings(),
		rCfg,
		sink,
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	recv := r.(*k8seventsReceiver)
	recv.ctx = context.Background()
	k8sEvent := getEvent()
	k8sEvent.FirstTimestamp = v1.Time{Time: time.Now().Add(-time.Hour)}
	recv.handleEvent(k8sEvent)

	assert.Equal(t, sink.LogRecordCount(), 0)
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
		receivertest.NewNopCreateSettings(),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	recv := r.(*k8seventsReceiver)
	k8sEvent := getEvent()

	shouldAllowEvent := recv.allowEvent(k8sEvent)
	assert.Equal(t, shouldAllowEvent, true)

	k8sEvent.FirstTimestamp = v1.Time{Time: time.Now().Add(-time.Hour)}
	shouldAllowEvent = recv.allowEvent(k8sEvent)
	assert.Equal(t, shouldAllowEvent, false)

	k8sEvent.FirstTimestamp = v1.Time{}
	shouldAllowEvent = recv.allowEvent(k8sEvent)
	assert.Equal(t, shouldAllowEvent, false)
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
