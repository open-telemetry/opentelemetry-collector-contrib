// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/filter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	apiWatch "k8s.io/apimachinery/pkg/watch"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sleaderelectortest"
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
			// include_initial_state defaults to false, no override needed
			rCfg.Objects = []*K8sObjectsConfig{
				{
					Name:     tt.objectName,
					Mode:     k8sinventory.PullMode,
					Interval: 30 * time.Second,
				},
			}
			r, err := newReceiver(
				receivertest.NewNopSettings(metadata.Type),
				rCfg,
				consumertest.NewNop(),
			)
			require.NoError(t, err)
			require.NotNil(t, r)
			err = r.Start(t.Context(), componenttest.NewNopHost())
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
			Name:     "pods",
			Mode:     k8sinventory.PullMode,
			Interval: 30 * time.Second,
		},
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
			Mode:          k8sinventory.PullMode,
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
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool {
		return consumer.Count() == 2
	}, 5*time.Second, 10*time.Millisecond)
	assert.Len(t, consumer.Logs(), 1)
	assert.NoError(t, r.Shutdown(t.Context()))
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
			Mode:       k8sinventory.WatchMode,
			Namespaces: []string{"default"},
		},
	}

	consumer := newMockLogConsumer()
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumer,
	)

	ctx := t.Context()
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))

	require.Never(t, func() bool { return consumer.Count() > 0 }, 100*time.Millisecond, 10*time.Millisecond)

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
	require.Eventually(t, func() bool {
		return consumer.Count() == 2
	}, 5*time.Second, 10*time.Millisecond)
	assert.Len(t, consumer.Logs(), 2)

	mockClient.deletePods(
		generatePod("pod2", "default", map[string]any{
			"environment": "test",
		}, "2"),
	)
	require.Eventually(t, func() bool {
		return consumer.Count() == 3
	}, 5*time.Second, 10*time.Millisecond)
	assert.Len(t, consumer.Logs(), 3)

	assert.NoError(t, r.Shutdown(ctx))
}

func TestIncludeInitialState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc                string
		includeInitialState *bool
		expectedInitialLogs int
		expectedWatchLogs   int
	}{
		{
			desc:                "include_initial_state true sends initial state",
			includeInitialState: func() *bool { b := true; return &b }(),
			expectedInitialLogs: 2, // 2 pods created initially
			expectedWatchLogs:   1, // 1 new pod created during watch
		},
		{
			desc:                "include_initial_state false skips initial state",
			includeInitialState: func() *bool { b := false; return &b }(),
			expectedInitialLogs: 0, // no initial state
			expectedWatchLogs:   1, // 1 new pod created during watch
		},
		{
			desc:                "include_initial_state nil defaults to false",
			includeInitialState: nil,
			expectedInitialLogs: 0, // default is false now
			expectedWatchLogs:   1, // 1 new pod created during watch
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockClient := newMockDynamicClient()
			// Create initial pods
			mockClient.createPods(
				generatePod("pod1", "default", map[string]any{
					"environment": "production",
				}, "1"),
				generatePod("pod2", "default", map[string]any{
					"environment": "test",
				}, "2"),
			)

			rCfg := createDefaultConfig().(*Config)
			rCfg.makeDynamicClient = mockClient.getMockDynamicClient
			rCfg.makeDiscoveryClient = getMockDiscoveryClient
			rCfg.ErrorMode = PropagateError
			if tt.includeInitialState != nil {
				rCfg.IncludeInitialState = *tt.includeInitialState
			}

			rCfg.Objects = []*K8sObjectsConfig{
				{
					Name:       "pods",
					Mode:       k8sinventory.WatchMode,
					Namespaces: []string{"default"},
				},
			}

			consumer := newMockLogConsumer()
			r, err := newReceiver(
				receivertest.NewNopSettings(metadata.Type),
				rCfg,
				consumer,
			)

			ctx := t.Context()
			require.NoError(t, err)
			require.NotNil(t, r)
			require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))

			if tt.expectedInitialLogs > 0 {
				require.Eventually(t, func() bool {
					return consumer.Count() == tt.expectedInitialLogs
				}, 5*time.Second, 10*time.Millisecond)
			} else {
				require.Never(t, func() bool { return consumer.Count() > 0 }, 100*time.Millisecond, 10*time.Millisecond)
			}

			mockClient.createPods(
				generatePod("pod3", "default", map[string]any{
					"environment": "production",
				}, "3"),
			)

			require.Eventually(t, func() bool {
				return consumer.Count() == tt.expectedInitialLogs+tt.expectedWatchLogs
			}, 5*time.Second, 10*time.Millisecond)

			logs := consumer.Logs()
			assert.NotEmpty(t, logs)

			for _, log := range logs {
				for i := 0; i < log.ResourceLogs().Len(); i++ {
					rl := log.ResourceLogs().At(i)
					for j := 0; j < rl.ScopeLogs().Len(); j++ {
						sl := rl.ScopeLogs().At(j)
						for k := 0; k < sl.LogRecords().Len(); k++ {
							record := sl.LogRecords().At(k)
							body := record.Body()
							assert.Equal(t, pcommon.ValueTypeMap, body.Type())

							bodyMap := body.Map()
							// Verify consistent structure: should have "type" and "object" fields
							_, hasType := bodyMap.Get("type")
							_, hasObject := bodyMap.Get("object")
							assert.True(t, hasType)
							assert.True(t, hasObject)

							// Verify event attributes are present
							attrs := record.Attributes()
							_, hasEventDomain := attrs.Get("event.domain")
							_, hasEventName := attrs.Get("event.name")
							assert.True(t, hasEventDomain)
							assert.True(t, hasEventName)
						}
					}
				}
			}

			assert.NoError(t, r.Shutdown(ctx))
		})
	}
}

func TestIncludeInitialStateWithPullMode(t *testing.T) {
	t.Parallel()

	core, observedLogs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = newMockDynamicClient().getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient
	rCfg.IncludeInitialState = true
	rCfg.ErrorMode = PropagateError

	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name:     "pods",
			Mode:     k8sinventory.PullMode,
			Interval: 30 * time.Second,
		},
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	r, err := newReceiver(settings, rCfg, consumertest.NewNop())
	require.NoError(t, err)

	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	_ = r.Shutdown(t.Context())

	warnMessages := make([]string, 0)
	for _, e := range observedLogs.FilterLevelExact(zap.WarnLevel).All() {
		warnMessages = append(warnMessages, e.Message)
	}
	assert.Contains(t, warnMessages, "include_initial_state has no effect in pull mode; it only applies to watch mode")
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
			Mode:       k8sinventory.WatchMode,
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

	ctx := t.Context()
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))

	require.Never(t, func() bool { return consumer.Count() > 0 }, 100*time.Millisecond, 10*time.Millisecond)

	mockClient.deletePods(
		generatePod("pod1", "default", map[string]any{
			"environment": "test",
		}, "1"),
	)
	require.Never(t, func() bool { return consumer.Count() > 0 }, 100*time.Millisecond, 10*time.Millisecond)

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
			Name:     "pods",
			Mode:     k8sinventory.PullMode,
			Interval: 30 * time.Second,
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

	err = kr.Start(t.Context(), fakeHost)
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

// TestNamespaceDenyListWithLeaderReelection verifies that excluded namespaces remain excluded
// after a leader re-election, and that re-election does not register duplicate event handlers
// (which would double-count events due to the same informer receiving the handler twice).
func TestNamespaceDenyListWithLeaderReelection(t *testing.T) {
	fakeLeaderElection := &k8sleaderelectortest.FakeLeaderElection{}
	fakeHost := &k8sleaderelectortest.FakeHost{FakeLeaderElection: fakeLeaderElection}
	leaderElectorID := component.MustNewID("k8s_leader_elector")

	mockClient := newMockDynamicClient()
	mockClient.createNamespaces(
		generateNamespace("default", "1"),
		generateNamespace("excluded", "2"),
	)

	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient
	rCfg.ErrorMode = PropagateError
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name: "pods",
			Mode: k8sinventory.WatchMode,
			ExcludeNamespaces: []filter.Config{
				{Regex: "excluded"},
			},
		},
	}
	rCfg.K8sLeaderElector = &leaderElectorID

	consumer := newMockLogConsumer()
	r, err := newReceiver(receivertest.NewNopSettings(metadata.Type), rCfg, consumer)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), fakeHost))

	// First election: create one pod in each namespace.
	fakeLeaderElection.InvokeOnLeading()
	mockClient.createPods(generatePod("pod1", "default", map[string]any{}, "1"))
	mockClient.createPods(generatePod("pod2", "excluded", map[string]any{}, "2"))
	require.Eventually(t, func() bool { return consumer.Count() == 1 }, 10*time.Second, 10*time.Millisecond,
		"pod1 from default not received")

	// Re-election: stop then re-lead.
	fakeLeaderElection.InvokeOnStopping()
	fakeLeaderElection.InvokeOnLeading()

	// Create one more pod in default after re-election.
	// If object.Namespaces was duplicated, the same informer would have two handlers registered,
	// causing pod3 to be counted twice instead of once.
	mockClient.createPods(generatePod("pod3", "default", map[string]any{}, "3"))
	mockClient.createPods(generatePod("pod4", "excluded", map[string]any{}, "4"))

	require.Eventually(t, func() bool { return consumer.Count() >= 2 }, 10*time.Second, 10*time.Millisecond,
		"pod3 from default not received after re-election")
	require.Never(t, func() bool { return consumer.Count() > 2 }, 200*time.Millisecond, 10*time.Millisecond,
		"unexpected extra events: duplicate namespace watcher or excluded namespace leak")
}

func TestNamespaceDenyListWatchObject(t *testing.T) {
	t.Parallel()

	mockClient := newMockDynamicClient()
	mockClient.createNamespaces(
		generateNamespace("default", "1"),
		generateNamespace("default_ignore", "2"),
	)

	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient
	rCfg.ErrorMode = PropagateError

	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name: "pods",
			Mode: k8sinventory.WatchMode,
			ExcludeNamespaces: []filter.Config{
				{
					Regex: "default_ignore",
				},
			},
		},
	}

	consumer := newMockLogConsumer()
	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumer,
	)

	ctx := t.Context()
	require.NoError(t, err)
	require.NotNil(t, r)
	require.NoError(t, r.Start(ctx, componenttest.NewNopHost()))

	require.Never(t, func() bool { return consumer.Count() > 0 }, 100*time.Millisecond, 10*time.Millisecond)

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
	require.Eventually(t, func() bool {
		return consumer.Count() == 2
	}, 5*time.Second, 10*time.Millisecond)
	assert.Len(t, consumer.Logs(), 2)

	assert.NoError(t, r.Shutdown(ctx))
}

func TestDeprecationWarningsForStorageAndResourceVersion(t *testing.T) {
	t.Parallel()

	core, observedLogs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	mockClient := newMockDynamicClient()
	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient
	rCfg.ErrorMode = PropagateError
	storageID := component.MustNewID("file_storage")
	rCfg.Storage = &storageID
	rCfg.Objects = []*K8sObjectsConfig{
		{
			Name:            "pods",
			Mode:            k8sinventory.WatchMode,
			ResourceVersion: "100",
		},
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	r, err := newReceiver(settings, rCfg, consumertest.NewNop())
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { _ = r.Shutdown(t.Context()) }()

	warnMessages := make([]string, 0)
	for _, e := range observedLogs.FilterLevelExact(zap.WarnLevel).All() {
		warnMessages = append(warnMessages, e.Message)
	}
	assert.Contains(t, warnMessages, "storage is no longer used; resourceVersion checkpointing is handled internally by the informer")
	assert.Contains(t, warnMessages, "resource_version is no longer used; the informer manages watch resumption internally")
}

// TestFactorySharing verifies that two object configs with identical
// (namespace, labelSelector, fieldSelector) tuples share a single
// SharedInformerFactory rather than creating one per object. This is
// the core scalability invariant: N objects with the same filter do not
// open N separate List+Watch connections to the API server.
func TestFactorySharing(t *testing.T) {
	t.Parallel()

	mockClient := newMockDynamicClient()

	rCfg := createDefaultConfig().(*Config)
	rCfg.makeDynamicClient = mockClient.getMockDynamicClient
	rCfg.makeDiscoveryClient = getMockDiscoveryClient
	rCfg.ErrorMode = PropagateError

	r, err := newReceiver(
		receivertest.NewNopSettings(metadata.Type),
		rCfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	kr := r.(*k8sobjectsreceiver)

	// Inject the client directly — normally set in Start(), not needed here.
	client, err := mockClient.getMockDynamicClient()
	require.NoError(t, err)
	kr.client = client

	sharedKey := factoryKey{namespace: "default", labelSelector: "env=prod", fieldSelector: ""}
	differentKey := factoryKey{namespace: "kube-system", labelSelector: "env=prod", fieldSelector: ""}

	f1a := kr.getOrCreateFactory(sharedKey)
	f1b := kr.getOrCreateFactory(sharedKey) // same tuple — must return same factory
	f2 := kr.getOrCreateFactory(differentKey)

	assert.Equal(t, f1a, f1b, "identical filter tuple should return the same factory instance")
	assert.NotEqual(t, f1a, f2, "different filter tuple should return a different factory instance")

	kr.factoriesMu.Lock()
	factoryCount := len(kr.factories)
	kr.factoriesMu.Unlock()
	assert.Equal(t, 2, factoryCount, "two distinct filter tuples should produce exactly two factories")
}
