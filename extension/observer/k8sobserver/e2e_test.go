// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sobserver

import (
	"context"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	testKubeConfig                = "/tmp/kube-config-otelcol-e2e-testing"
	namespacedCollectorObjectsDir = "./testdata/e2e/namespaced/collector/"
	ephemeralCollectorObjectsDir  = "./testdata/e2e/ephemeral/collector/"
	ephemeralWorkloadsDir         = "./testdata/e2e/ephemeral/workloads/"
	ephemeralPod                  = "k8sobserver-e2e-ephemeral"
	initCollectorObjectsDir       = "./testdata/e2e/init/collector/"
	initWorkloadsDir              = "./testdata/e2e/init/workloads/"
	initPod                       = "k8sobserver-e2e-init"
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

// containerLogs is what was received from one container.
type containerLogs struct {
	// lines counts how many times each line was received, by its sequence number.
	lines map[string]int
	// kind is the kind of the container as the observer reports it.
	kind string
}

// receivedContainerLogs groups the received log records of a pod by container.
func receivedContainerLogs(sink *consumertest.LogsSink, podName string) map[string]*containerLogs {
	containers := map[string]*containerLogs{}
	for _, logs := range sink.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			pod, hasPod := rl.Resource().Attributes().Get("k8s.pod.name")
			container, hasContainer := rl.Resource().Attributes().Get("k8s.container.name")
			if !hasPod || !hasContainer || pod.Str() != podName {
				continue
			}
			received, ok := containers[container.Str()]
			if !ok {
				received = &containerLogs{lines: map[string]int{}}
				containers[container.Str()] = received
			}
			if kind, hasKind := rl.Resource().Attributes().Get("e2e.container.kind"); hasKind {
				received.kind = kind.Str()
			}
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				records := rl.ScopeLogs().At(j).LogRecords()
				for k := 0; k < records.Len(); k++ {
					if seq, hasSeq := records.At(k).Attributes().Get("seq"); hasSeq {
						received.lines[seq.Str()]++
					}
				}
			}
		}
	}
	return containers
}

// TestE2EEphemeralContainers adds an ephemeral container to a running pod, as kubectl debug
// does, and collects its logs with a file log receiver that the receiver creator starts for the
// endpoint of the container.
func TestE2EEphemeralContainers(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	logsConsumer := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, logsConsumer)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.WithoutCancel(t.Context())))
	}()

	workloadObjs, err := k8stest.CreateObjects(k8sClient, ephemeralWorkloadsDir)
	require.NoError(t, err, "failed creating the workload")
	defer func() {
		assert.NoError(t, k8stest.DeleteObjects(k8sClient, workloadObjs))
	}()

	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, uuid.NewString()[:8], ephemeralCollectorObjectsDir, map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// The app container is collected before the ephemeral container exists.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		app, ok := receivedContainerLogs(logsConsumer, ephemeralPod)["app"]
		if assert.True(c, ok, "no logs received for the app container") {
			assert.Len(c, app.lines, 5)
		}
	}, 3*time.Minute, time.Second, "did not receive the logs of the app container")

	pods := k8sClient.DynamicClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: "pods"}).Namespace("default")
	ephemeralContainer := `{"spec":{"ephemeralContainers":[{"name":"debugger","image":"busybox:1.37.0","terminationMessagePolicy":"File",` +
		`"command":["/bin/sh","-c","for i in 0 1 2 3 4; do echo seq=$i; done; sleep 3600"]}]}}`
	_, err = pods.Patch(t.Context(), ephemeralPod, types.StrategicMergePatchType, []byte(ephemeralContainer), metav1.PatchOptions{}, "ephemeralcontainers")
	require.NoError(t, err, "failed adding the ephemeral container")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		debugger, ok := receivedContainerLogs(logsConsumer, ephemeralPod)["debugger"]
		if assert.True(c, ok, "no logs received for the ephemeral container") {
			assert.Len(c, debugger.lines, 5)
		}
	}, 3*time.Minute, time.Second, "did not receive the logs of the ephemeral container")

	// Give a receiver that was started a second time for a container the time to show.
	time.Sleep(10 * time.Second)

	received := receivedContainerLogs(logsConsumer, ephemeralPod)
	for container, kind := range map[string]string{"app": "app", "debugger": "ephemeral"} {
		got := received[container]
		require.NotNilf(t, got, "container %s", container)
		assert.Lenf(t, got.lines, 5, "lines of %s", container)
		assert.Equalf(t, kind, got.kind, "kind of %s", container)
		for line, times := range got.lines {
			assert.Equalf(t, 1, times, "line %s of %s was received %d times", line, container, times)
		}
	}
}

// TestE2EInitContainers collects the logs of an init container, a native sidecar and an app
// container with file log receivers that the receiver creator starts for the endpoints of the
// containers. The collector runs before the pod, as the init container is reported while it runs.
func TestE2EInitContainers(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.Protocols.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
	logsConsumer := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(f.Type()), cfg, logsConsumer)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.WithoutCancel(t.Context())))
	}()

	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, uuid.NewString()[:8], initCollectorObjectsDir, map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	workloadObjs, err := k8stest.CreateObjects(k8sClient, initWorkloadsDir)
	require.NoError(t, err, "failed creating the workload")
	defer func() {
		assert.NoError(t, k8stest.DeleteObjects(k8sClient, workloadObjs))
	}()

	expected := map[string]string{"init": "init", "sidecar": "init", "app": "app"}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		received := receivedContainerLogs(logsConsumer, initPod)
		for container := range expected {
			got, ok := received[container]
			if assert.Truef(c, ok, "no logs received for %s", container) {
				assert.Lenf(c, got.lines, 5, "lines of %s", container)
			}
		}
	}, 3*time.Minute, time.Second, "did not receive the logs of every container")

	// Give a receiver that was started a second time for a container the time to show.
	time.Sleep(10 * time.Second)

	received := receivedContainerLogs(logsConsumer, initPod)
	for container, kind := range expected {
		got := received[container]
		require.NotNilf(t, got, "container %s", container)
		assert.Lenf(t, got.lines, 5, "lines of %s", container)
		assert.Equalf(t, kind, got.kind, "kind of %s", container)
		for line, times := range got.lines {
			assert.Equalf(t, 1, times, "line %s of %s was received %d times", line, container, times)
		}
	}
}
