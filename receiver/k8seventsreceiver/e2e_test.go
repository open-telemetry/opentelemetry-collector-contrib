// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8seventsreceiver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"
	testObjectsDir = "./testdata/e2e/testobjects/"
	expectedDir    = "./testdata/e2e/expected/"
)

func getOrInsertDefault[T any](t *testing.T, opt *configoptional.Optional[T]) *T {
	if opt.HasValue() {
		return opt.Get()
	}

	empty := confmap.NewFromStringMap(map[string]any{})
	require.NoError(t, empty.Unmarshal(opt))
	val := opt.Get()
	require.NotNil(t, "Expected a default value to be set for %T", val)
	return val
}

func TestE2E(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]

	nsObj, err := os.ReadFile(filepath.Join(testObjectsDir, "namespace.yaml"))
	require.NoError(t, err, "failed to read namespace.yaml")
	testNS, err := k8stest.CreateObject(k8sClient, nsObj)
	require.NoError(t, err, "failed to create test namespace")
	defer func() {
		require.NoErrorf(t, k8stest.DeleteObject(k8sClient, testNS), "failed to delete test namespace")
	}()

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	logsConsumer := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, logsConsumer)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, "",
		map[string]string{"Namespace": "test-k8sevents-e2e"}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	t.Run("watch events baseline", func(t *testing.T) {
		expected, err := golden.ReadLogs(filepath.Join(expectedDir, "watch_events.yaml"))
		require.NoError(t, err, "failed to read expected logs")

		evObj, err := os.ReadFile(filepath.Join(testObjectsDir, "event.yaml"))
		require.NoError(t, err, "failed to read event.yaml")
		testObjs := make([]*unstructured.Unstructured, 0, 1)
		newObj, err := k8stest.CreateObject(k8sClient, evObj)
		require.NoError(t, err, "failed to create test event")
		testObjs = append(testObjs, newObj)
		defer func() {
			for _, obj := range testObjs {
				require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
			}
		}()

		compareOpts := []plogtest.CompareLogsOption{
			plogtest.IgnoreTimestamp(),
			plogtest.IgnoreObservedTimestamp(),
			plogtest.IgnoreLogRecordAttributeValue("k8s.event.uid"),
			plogtest.IgnoreLogRecordAttributeValue("k8s.event.start_time"),
			plogtest.IgnoreResourceAttributeValue("k8s.object.resource_version"),
			plogtest.IgnoreResourceLogsOrder(),
			plogtest.IgnoreScopeLogsOrder(),
			plogtest.IgnoreLogRecordsOrder(),
			plogtest.IgnoreScopeLogsVersion(),
		}

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			if len(logsConsumer.AllLogs()) == 0 {
				assert.Fail(c, "no logs received yet")
				return
			}
			for _, logs := range logsConsumer.AllLogs() {
				if err := plogtest.CompareLogs(expected, logs, compareOpts...); err == nil {
					return
				}
			}
			assert.Failf(c, "matching logs not found", "total batches: %d", len(logsConsumer.AllLogs()))
		}, 3*time.Minute, 5*time.Second, "timeout waiting for matching event logs")
	})
}

func countEventLogs(allLogs []plog.Logs, eventName string) int {
	count := 0
	for _, logs := range allLogs {
		for i := range logs.ResourceLogs().Len() {
			rl := logs.ResourceLogs().At(i)
			for j := range rl.ScopeLogs().Len() {
				sl := rl.ScopeLogs().At(j)
				for k := range sl.LogRecords().Len() {
					lr := sl.LogRecords().At(k)
					nameVal, nameOK := lr.Attributes().Get("k8s.event.name")
					if nameOK && nameVal.Str() == eventName {
						count++
					}
				}
			}
		}
	}
	return count
}

// TestE2EDedup creates events.k8s.io/v1 events programmatically and patches them
// repeatedly to produce MODIFIED watch notifications that dedup_interval throttles.
func TestE2EDedup(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	objectsDir := "./testdata/e2e/dedup/testobjects/"

	testID := uuid.NewString()[:8]

	nsObj, err := os.ReadFile(filepath.Join(objectsDir, "namespace.yaml"))
	require.NoError(t, err, "failed to read namespace.yaml")
	testNS, err := k8stest.CreateObject(k8sClient, nsObj)
	require.NoError(t, err, "failed to create test namespace")
	defer func() {
		require.NoErrorf(t, k8stest.DeleteObject(k8sClient, testNS), "failed to delete test namespace")
	}()

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	logsConsumer := new(consumertest.LogsSink)
	metricsConsumer := new(consumertest.MetricsSink)
	logsRcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, logsConsumer)
	require.NoError(t, err, "failed creating logs receiver")
	_, err = f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, metricsConsumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, logsRcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, logsRcvr.Shutdown(context.Background()))
	}()

	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, "",
		map[string]string{
			"Namespace":             "test-k8sevents-dedup",
			"DedupInterval":         "5s",
			"ExportInternalMetrics": "true",
		}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	eventsGVR := schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}
	evClient := k8sClient.DynamicClient.Resource(eventsGVR).Namespace("test-k8sevents-dedup")

	// Create an events.k8s.io/v1 Event with a far-future eventTime so allowEvent
	// always passes regardless of clock skew between the CI runner and the pod.
	eventObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "events.k8s.io/v1",
			"kind":       "Event",
			"metadata": map[string]any{
				"name":      "test-dedup-unhealthy",
				"namespace": "test-k8sevents-dedup",
			},
			"regarding": map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"name":       "failing-readiness",
				"namespace":  "test-k8sevents-dedup",
			},
			"reason":              "Unhealthy",
			"note":                "Readiness probe failed: connection refused",
			"type":                "Warning",
			"reportingController": "kubelet",
			"reportingInstance":   "test-node",
			"action":              "HealthCheck",
			"eventTime":           "2099-01-01T00:00:00.000000Z",
		},
	}
	created, err := evClient.Create(context.Background(), eventObj, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create dedup test event")
	defer func() {
		_ = evClient.Delete(context.Background(), created.GetName(), metav1.DeleteOptions{})
	}()

	// Patch the event 25 times with 200ms gaps to generate MODIFIED watch events.
	// dedup_interval=5s means the first MODIFIED is emitted and the rest are
	// throttled until 5s elapses — far fewer than 25 records should reach the consumer.
	// Use a far-future lastObservedTime so allowEvent passes regardless of clock skew.
	const patchCount = 25
	for i := range patchCount {
		time.Sleep(200 * time.Millisecond)
		patch := fmt.Sprintf(`{"series":{"count":%d,"lastObservedTime":"2099-01-01T00:01:00.000000Z"}}`, i+2)
		if _, patchErr := evClient.Patch(context.Background(), created.GetName(), types.MergePatchType, []byte(patch), metav1.PatchOptions{}); patchErr != nil {
			t.Logf("patch %d: %v", i+1, patchErr)
		}
	}

	// Drain in-flight watch events before sampling.
	time.Sleep(3 * time.Second)
	receiverCount := countEventLogs(logsConsumer.AllLogs(), created.GetName())
	t.Logf("dedup — patches=%d  receiver records=%d  dedup_interval=5s", patchCount, receiverCount)

	require.GreaterOrEqual(t, receiverCount, 1, "expected at least the ADDED record")
	require.Less(t, receiverCount, patchCount+1,
		"dedup didn't throttle: %d patches but receiver emitted %d records", patchCount, receiverCount)

	// The receiver pushes its internal metrics via the configured periodic
	// OTLP reader; the latest cumulative datapoint must reflect filtering.
	var filtered int64
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		v, ok := latestFilteredCount(metricsConsumer.AllMetrics())
		if !assert.True(c, ok, "otelcol.k8s.events.modified.filtered not yet exported") {
			return
		}
		filtered = v
		assert.Greaterf(c, v, int64(0), "filtered counter should be > 0, got %d", v)
	}, 30*time.Second, 1*time.Second)
	t.Logf("dedup — filtered counter=%d", filtered)
}

func latestFilteredCount(allMetrics []pmetric.Metrics) (int64, bool) {
	var (
		latest   int64
		latestTS pcommon.Timestamp
		seenAny  bool
	)
	for _, m := range allMetrics {
		for i := range m.ResourceMetrics().Len() {
			rm := m.ResourceMetrics().At(i)
			for j := range rm.ScopeMetrics().Len() {
				sm := rm.ScopeMetrics().At(j)
				for k := range sm.Metrics().Len() {
					metric := sm.Metrics().At(k)
					if metric.Name() != "otelcol.k8s.events.modified.filtered" || metric.Type() != pmetric.MetricTypeSum {
						continue
					}
					dps := metric.Sum().DataPoints()
					for d := range dps.Len() {
						dp := dps.At(d)
						if !seenAny || dp.Timestamp() > latestTS {
							latest = dp.IntValue()
							latestTS = dp.Timestamp()
							seenAny = true
						}
					}
				}
			}
		}
	}
	return latest, seenAny
}
