// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	apiWatch "k8s.io/apimachinery/pkg/watch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector/k8sleaderelectortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/internal/metadata"
)

func TestErrorModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc          string
		errorMode     ErrorMode
		objectName    string
		expectError   bool
		expectedError string
	}{
		{
			desc:          "propagate error mode returns error for invalid resource",
			errorMode:     PropagateError,
			objectName:    "nonexistent-resource",
			expectError:   true,
			expectedError: "resource not found: nonexistent-resource",
		},
		{
			desc:        "ignore error mode continues for invalid resource with valid fallback",
			errorMode:   IgnoreError,
			objectName:  "pods",
			expectError: false,
		},
		{
			desc:        "silent error mode continues for invalid resource with valid fallback",
			errorMode:   SilentError,
			objectName:  "pods",
			expectError: false,
		},
		{
			desc:          "ignore error mode fails when no valid objects found",
			errorMode:     IgnoreError,
			objectName:    "nonexistent-resource",
			expectError:   true,
			expectedError: "no valid Kubernetes objects found to watch",
		},
		{
			desc:          "silent error mode fails when no valid objects found",
			errorMode:     SilentError,
			objectName:    "nonexistent-resource",
			expectError:   true,
			expectedError: "no valid Kubernetes objects found to watch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockClient := newMockDynamicClient()
			rCfg := createDefaultConfig().(*Config)
			rCfg.makeDynamicClient = mockClient.getMockDynamicClient
			rCfg.makeDiscoveryClient = getMockDiscoveryClient
			rCfg.ErrorMode = tt.errorMode
			rCfg.Objects = []*K8sObjectsConfig{
				{
					Name: tt.objectName,
					Mode: PullMode,
				},
			}

			r, err := newReceiver(
				receivertest.NewNopSettings(metadata.Type),
				rCfg,
				consumertest.NewNop(),
			)
			require.NoError(t, err)
			require.NotNil(t, r)
			err = r.Start(context.Background(), componenttest.NewNopHost())
			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewReceiver(t *testing.T) {
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
	rCfg.ErrorMode = PropagateError
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name: "pods",
			Mode: PullMode,
		},
	}

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
	rCfg.ErrorMode = PropagateError

	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name:          "pods",
			Mode:          PullMode,
			Interval:      time.Second * 30,
			LabelSelector: "environment=production",
		},
	}

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
	rCfg.ErrorMode = PropagateError

	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name:       "pods",
			Mode:       WatchMode,
			Namespaces: []string{"default"},
		},
	}

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
	rCfg.ErrorMode = PropagateError

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

func TestReceiverWithLeaderElection(t *testing.T) {
	fakeLeaderElection := &k8sleaderelectortest.FakeLeaderElection{}
	fakeHost := &k8sleaderelectortest.FakeHost{
		FakeLeaderElection: fakeLeaderElection,
	}
	leaderElectorID := component.MustNewID("k8s_leader_elector")

	mockClient := newMockDynamicClient()
	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient
	rCfg.ErrorMode = PropagateError
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name: "pods",
			Mode: PullMode,
		},
	}
	rCfg.K8sLeaderElector = &leaderElectorID

	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	kr := r.(*k8sobjectsreceiver)
	sink := new(consumertest.LogsSink)
	kr.consumer = sink

	// Setup k8s resources.
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	err = kr.Start(context.Background(), fakeHost)
	require.NoError(t, err)

	// elected leader
	fakeLeaderElection.InvokeOnLeading()

	require.Eventually(t, func() bool {
		// expect get 2 log records
		return sink.LogRecordCount() == 1
	}, 20*time.Second, 100*time.Millisecond,
		"logs not collected")

	// lost election
	fakeLeaderElection.InvokeOnStopping()

	// mock create pod again which not collected
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	// get back election
	fakeLeaderElection.InvokeOnLeading()

	// mock create pod finally
	mockClient.createPods(
		generatePod("pod1", "default", map[string]any{
			"environment": "production",
		}, "1"),
	)

	require.Eventually(t, func() bool {
		// expect get 4 log records
		return sink.LogRecordCount() == 2
	}, 20*time.Second, 100*time.Millisecond,
		"logs not collected")
}
