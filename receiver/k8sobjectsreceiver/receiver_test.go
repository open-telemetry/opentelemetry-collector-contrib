// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	apiWatch "k8s.io/apimachinery/pkg/watch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/internal/metadata"
)

func TestNewReceiver(t *testing.T) {
	t.Parallel()

	mockClient := newMockDynamicClient()
	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient

	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestPullObject(t *testing.T) {
	t.Parallel()

	mockClient := newMockDynamicClient()
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
		generatePod("pod2", "default", map[string]any{
			"environment": "test",
		}, "2"),
		generatePod("pod3", "default_ignore", map[string]any{
			"environment": "production",
		}, "3"),
	)

	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient

	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name:          "pods",
			Mode:          PullMode,
			Interval:      time.Second * 30,
			LabelSelector: "environment=production",
		},
	}

	err := rCfg.Validate()
	require.NoError(t, err)

	consumer := newMockLogConsumer()
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumer,
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
	time.Sleep(time.Second)
	assert.Len(t, consumer.Logs(), 1)
	assert.Equal(t, 2, consumer.Count())
	assert.NoError(t, r.Shutdown(context.Background()))
}

func TestWatchObject(t *testing.T) {
	t.Parallel()

	mockClient := newMockDynamicClient()

	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient

	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name:       "pods",
			Mode:       WatchMode,
			Namespaces: []string{"default"},
		},
	}

	err := rCfg.Validate()
	require.NoError(t, err)

	consumer := newMockLogConsumer()
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumer,
	)

	ctx := context.Background()
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))

	time.Sleep(time.Millisecond * 100)
	assert.Empty(t, consumer.Logs())
	assert.Equal(t, 0, consumer.Count())

	mockClient.createPods(
		generatePod("pod2", "default", map[string]any{
			"environment": "test",
		}, "2"),
		generatePod("pod3", "default_ignore", map[string]any{
			"environment": "production",
		}, "3"),
		generatePod("pod4", "default", map[string]any{
			"environment": "production",
		}, "4"),
	)
	time.Sleep(time.Millisecond * 100)
	assert.Len(t, consumer.Logs(), 2)
	assert.Equal(t, 2, consumer.Count())

	mockClient.deletePods(
		generatePod("pod2", "default", map[string]any{
			"environment": "test",
		}, "2"),
	)
	time.Sleep(time.Millisecond * 100)
	assert.Len(t, consumer.Logs(), 3)
	assert.Equal(t, 3, consumer.Count())

	assert.NoError(t, r.Shutdown(ctx))
}

func TestExcludeDeletedTrue(t *testing.T) {
	t.Parallel()

	mockClient := newMockDynamicClient()

	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient

	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name:       "pods",
			Mode:       WatchMode,
			Namespaces: []string{"default"},
			ExcludeWatchType: []apiWatch.EventType{
				apiWatch.Deleted,
			},
		},
	}

	err := rCfg.Validate()
	require.NoError(t, err)

	consumer := newMockLogConsumer()
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumer,
	)

	ctx := context.Background()
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))

	time.Sleep(time.Millisecond * 100)
	assert.Empty(t, consumer.Logs())
	assert.Equal(t, 0, consumer.Count())

	mockClient.deletePods(
		generatePod("pod1", "default", map[string]any{
			"environment": "test",
		}, "1"),
	)
	time.Sleep(time.Millisecond * 100)
	assert.Empty(t, consumer.Logs())
	assert.Equal(t, 0, consumer.Count())

	assert.NoError(t, r.Shutdown(ctx))
}
