// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package kubeletstatsreceiver

import (
	"context"
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
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"

// getOrInsertDefault is a helper function to get or insert a default value for a configoptional.Optional type.
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
	expectedFile := filepath.Join("testdata", "e2e", "expected.yaml")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, "", map[string]string{}, "")

	defer func() {
		for _, obj := range append(collectorObjs) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10 // Minimal number of metrics to wait for.
	waitForData(t, wantEntries, metricsConsumer)

	actualMetrics := metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]
	require.NoError(t, pmetricassert.AssertMetrics(expectedFile, actualMetrics))
}

// TestE2EAllMetricGroups runs the receiver with every metric group, the extra
// metadata labels and the opt-in utilization metrics (including the node
// informer behind *.node.utilization), and checks them for a known workload pod.
func TestE2EAllMetricGroups(t *testing.T) {
	expectedFile := filepath.Join("testdata", "e2e", "all-groups", "expected.yaml")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	testObjs, err := k8stest.CreateObjects(k8sClient, filepath.Join(".", "testdata", "e2e", "all-groups", "testobjects"))
	require.NoErrorf(t, err, "failed to create test objects")
	defer func() {
		for _, obj := range testObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
			// The test pod is shared with the semconv cases that run next. Wait
			// until it is gone, otherwise they fail to create it while it is
			// terminating.
			waitForPodDeleted(t, k8sClient, obj.GetNamespace(), obj.GetName())
		}
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "all-groups", "collector"), map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
		// Late batches from this collector must not reach the sink of the next
		// case, which would see the same container metrics.
		waitForCollectorPodsDeleted(t, k8sClient, "default", "otelcol-"+testID)
	}()

	// waitForData waits for more than wantEntries batches: 4 batches, about 80s
	// at the 20s collection interval. The scrape-based CPU usage only needs one
	// earlier scrape of the same container.
	wantEntries := 3 // Minimal number of metrics to wait for.
	waitForData(t, wantEntries, metricsConsumer)

	// Assert on the latest batch until it matches: a single scrape can lack a
	// metric (for example a duplicate kubelet CPU sample skips CPU usage), and
	// batches from a previous test's collector that is still terminating can
	// reach the shared sink.
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		all := metricsConsumer.AllMetrics()
		actualMetrics := all[len(all)-1]
		// To regenerate: uncomment, run the test once, re-comment.
		// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedFile, actualMetrics))
		assert.NoError(tt, pmetricassert.AssertMetrics(expectedFile, actualMetrics))
	}, 2*time.Minute, 5*time.Second)
}

// TestE2ESemConvV0Schema runs the receiver with both semconv feature gates
// explicitly disabled and checks that container.memory.* are emitted as the v0
// gauges.
func TestE2ESemConvV0Schema(t *testing.T) {
	runSemConvE2E(t, "v0schema")
}

// TestE2ESemConvBothSchema runs the receiver with EmitV1K8sConventions enabled
// and DontEmitV0K8sConventions disabled (double publish). The migrated
// container.memory.* metrics keep their names and only change type, so the v1
// sum is emitted and the v0 gauge is dropped to avoid duplicate metric names.
func TestE2ESemConvBothSchema(t *testing.T) {
	runSemConvE2E(t, "bothschema")
}

// TestE2ESemConvV1Schema runs the receiver with EmitV1K8sConventions and
// DontEmitV0K8sConventions both enabled and checks that only the v1
// container.memory.* sums are emitted.
func TestE2ESemConvV1Schema(t *testing.T) {
	runSemConvE2E(t, "v1schema")
}

func runSemConvE2E(t *testing.T, name string) {
	testDir := filepath.Join(".", "testdata", "e2e", name)
	expectedFile := filepath.Join(testDir, "expected.yaml")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	testObjs, err := k8stest.CreateObjects(k8sClient, filepath.Join(".", "testdata", "e2e", "all-groups", "testobjects"))
	require.NoErrorf(t, err, "failed to create test objects")
	defer func() {
		for _, obj := range testObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
			// The test pod is shared by the semconv cases. Wait until it is gone,
			// otherwise the next case fails to create it while it is terminating.
			waitForPodDeleted(t, k8sClient, obj.GetNamespace(), obj.GetName())
		}
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"), map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
		// Wait until the collector pods are gone, so that late batches from this
		// collector cannot reach the sink of the next case and satisfy its
		// assertion before its own collector has sent anything.
		waitForCollectorPodsDeleted(t, k8sClient, "default", "otelcol-"+testID)
	}()

	waitForData(t, 3, metricsConsumer)

	// Batches from a previous test's collector that is still terminating can
	// reach the shared sink, so assert on the latest batch until it matches.
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		all := metricsConsumer.AllMetrics()
		assert.NoError(tt, pmetricassert.AssertMetrics(expectedFile, all[len(all)-1]))
	}, 2*time.Minute, 5*time.Second)
}

func waitForPodDeleted(t *testing.T, c *k8stest.K8sClient, namespace, name string) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	assert.Eventuallyf(t, func() bool {
		_, err := c.DynamicClient.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 2*time.Minute, time.Second, "pod %s/%s was not deleted", namespace, name)
}

func waitForCollectorPodsDeleted(t *testing.T, c *k8stest.K8sClient, namespace, instance string) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	assert.Eventuallyf(t, func() bool {
		pods, err := c.DynamicClient.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/instance=" + instance,
		})
		return err == nil && len(pods.Items) == 0
	}, 2*time.Minute, time.Second, "collector pods of %s were not deleted", instance)
}

func startUpSink(t *testing.T, mc *consumertest.MetricsSink) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	rcvr, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, mc)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	return func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}
}

func waitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
}
