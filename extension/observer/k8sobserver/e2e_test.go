// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sobserver

import (
	"context"
	"os/exec"
	"strings"
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	testKubeConfig                = "/tmp/kube-config-otelcol-e2e-testing"
	namespacedCollectorObjectsDir = "./testdata/e2e/namespaced/collector/"
	missedDeleteWorkloadDir       = "./testdata/e2e/missed_delete/workload/"
	missedDeletePodName           = "k8sobserver-e2e-missed-delete"
	// apiServerBlockDuration is longer than the time after which the client of the collector
	// gives up on a connection that does not answer.
	apiServerBlockDuration = 90 * time.Second
	// receiverStoppedAfter is several scrape intervals without metrics of the deleted pod.
	receiverStoppedAfter = 45 * time.Second
)

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

func TestE2ENamespaced(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	metricsConsumer := new(consumertest.MetricsSink)

	rcvr, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, metricsConsumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	// startup collector in k8s cluster
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, namespacedCollectorObjectsDir, map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// verify that we eventually receive metrics by the receiver created via the receiver creator receiving the endpoint information from the k8sobserver
	require.Eventuallyf(t, func() bool {
		for _, metric := range metricsConsumer.AllMetrics() {
			for i := 0; i < metric.ResourceMetrics().Len(); i++ {
				res := metric.ResourceMetrics().At(i)
				for j := 0; j < res.ScopeMetrics().Len(); j++ {
					if res.ScopeMetrics().At(j).Scope().Name() == "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver" {
						return true
					}
				}
			}
		}
		return false
	}, 1*time.Minute, 1*time.Second,
		"Timeout: failed to receive metrics in 1 minute")
}

// TestE2EMissedDelete deletes a pod while the collector cannot reach the API server. The
// informer of the observer misses the delete event and learns that the pod is gone when it
// lists the pods again. The receiver started for the pod has to stop then.
func TestE2EMissedDelete(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	metricsConsumer := new(consumertest.MetricsSink)
	rcvr, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, metricsConsumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.WithoutCancel(t.Context())))
	}()

	workloadObjs, err := k8stest.CreateObjects(k8sClient, missedDeleteWorkloadDir)
	require.NoError(t, err, "failed creating the workload")
	workloadDeleted := false
	defer func() {
		if !workloadDeleted {
			assert.NoError(t, k8stest.DeleteObjects(k8sClient, workloadObjs))
		}
	}()

	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, uuid.NewString()[:8], namespacedCollectorObjectsDir, map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// The prometheus receiver started for the pod reports the scrape of the pod, also when
	// nothing answers, so its metrics show that the receiver runs.
	podMetrics := func() int {
		count := 0
		for _, metrics := range metricsConsumer.AllMetrics() {
			for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
				name, ok := metrics.ResourceMetrics().At(i).Resource().Attributes().Get("k8s.pod.name")
				if ok && name.Str() == missedDeletePodName {
					count++
				}
			}
		}
		return count
	}
	require.Eventually(t, func() bool {
		return podMetrics() > 0
	}, 2*time.Minute, time.Second, "no receiver was started for the pod")

	pods := k8sClient.DynamicClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace("default")
	collectors, err := pods.List(t.Context(), metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=opentelemetry-collector"})
	require.NoError(t, err)
	require.Len(t, collectors.Items, 1)
	collectorIP, _, err := unstructured.NestedString(collectors.Items[0].Object, "status", "podIP")
	require.NoError(t, err)
	require.NotEmpty(t, collectorIP)

	unblock := blockAPIServer(t, collectorIP)
	defer unblock()

	require.NoError(t, k8stest.DeleteObjects(k8sClient, workloadObjs))
	workloadDeleted = true
	require.Eventually(t, func() bool {
		_, getErr := pods.Get(t.Context(), missedDeletePodName, metav1.GetOptions{})
		return apierrors.IsNotFound(getErr)
	}, time.Minute, time.Second, "the pod was not deleted")

	time.Sleep(apiServerBlockDuration)
	unblock()

	// The metrics of the pod stop once the receiver is stopped.
	last, lastChange := podMetrics(), time.Now()
	require.Eventually(t, func() bool {
		if current := podMetrics(); current != last {
			last, lastChange = current, time.Now()
		}
		return time.Since(lastChange) > receiverStoppedAfter
	}, 4*time.Minute, time.Second, "the receiver of the deleted pod still runs")
}

// blockAPIServer drops what the given pod sends to the API server, on every node of the kind
// clusters on this machine. The returned function removes the rules and can be called twice.
func blockAPIServer(t *testing.T, podIP string) func() {
	out, err := exec.CommandContext(t.Context(), "docker", "ps", "--filter", "label=io.x-k8s.kind.cluster", "--format", "{{.Names}}").Output()
	require.NoError(t, err, "failed listing the kind nodes")
	nodes := strings.Fields(string(out))
	require.NotEmpty(t, nodes, "no kind node found")

	iptables := func(action string) {
		for _, node := range nodes {
			for _, chain := range []string{"INPUT", "FORWARD"} {
				cmd := exec.CommandContext(context.WithoutCancel(t.Context()), "docker", "exec", node,
					"iptables", action, chain, "-s", podIP, "-p", "tcp", "--dport", "6443", "-j", "DROP")
				cmdOut, cmdErr := cmd.CombinedOutput()
				require.NoErrorf(t, cmdErr, "iptables %s %s on %s: %s", action, chain, node, cmdOut)
			}
		}
	}

	iptables("-I")
	blocked := true
	return func() {
		if blocked {
			blocked = false
			iptables("-D")
		}
	}
}
