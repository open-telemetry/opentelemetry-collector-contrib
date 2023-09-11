// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"
)

const (
	equal = iota
	regex
	exist
)

const testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"

type expectedValue struct {
	mode  int
	value string
}

func newExpectedValue(mode int, value string) *expectedValue {
	return &expectedValue{
		mode:  mode,
		value: value,
	}
}

// TestE2E tests the telemetrygen with a real k8s cluster.
// The test requires a prebuilt telemetrygen image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2E(t *testing.T) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, dynamicClient, testID)
	telemetryGenObjs, telemetryGenObjInfos := k8stest.CreateTelemetryGenObjects(t, dynamicClient, testID)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, k8stest.DeleteObject(dynamicClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		k8stest.WaitForTelemetryGenToStart(t, dynamicClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	wantEntries := 128 // Minimal number of metrics/traces/logs to wait for.
	waitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer)

	tcs := []struct {
		name     string
		dataType component.DataType
		service  string
		attrs    map[string]*expectedValue
	}{
		{
			name:     "traces-statefulset",
			dataType: component.DataTypeTraces,
			service:  "test-traces-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset-0"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.statefulset.name":     newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.statefulset.uid":      newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "metrics-statefulset",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset-0"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.statefulset.name":     newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.statefulset.uid":      newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "logs-statefulset",
			dataType: component.DataTypeLogs,
			service:  "test-logs-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset-0"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.statefulset.name":     newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.statefulset.uid":      newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.dataType {
			case component.DataTypeTraces:
				scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
			case component.DataTypeMetrics:
				scanMetricsForAttributes(t, metricsConsumer, tc.service, tc.attrs)
			case component.DataTypeLogs:
				scanLogsForAttributes(t, logsConsumer, tc.service, tc.attrs)
			default:
				t.Fatalf("unknown data type %s", tc.dataType)
			}
		})
	}
}

func scanTracesForAttributes(t *testing.T, ts *consumertest.TracesSink, expectedService string,
	kvs map[string]*expectedValue) {
	for i := 0; i < len(ts.AllTraces()); i++ {
		traces := ts.AllTraces()[i]
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			resource := traces.ResourceSpans().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "span do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			return
		}
	}
	t.Fatalf("no spans found for service %s", expectedService)
}

func scanMetricsForAttributes(t *testing.T, ms *consumertest.MetricsSink, expectedService string,
	kvs map[string]*expectedValue) {
	for i := 0; i < len(ms.AllMetrics()); i++ {
		metrics := ms.AllMetrics()[i]
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			resource := metrics.ResourceMetrics().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "metric do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			return
		}
	}
	t.Fatalf("no metric found for service %s", expectedService)
}

func scanLogsForAttributes(t *testing.T, ls *consumertest.LogsSink, expectedService string,
	kvs map[string]*expectedValue) {
	for i := 0; i < len(ls.AllLogs()); i++ {
		logs := ls.AllLogs()[i]
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			resource := logs.ResourceLogs().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "log do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			return
		}
	}
	t.Fatalf("no logs found for service %s", expectedService)
}

func waitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink, tc *consumertest.TracesSink, lc *consumertest.LogsSink) {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	_, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, err, "failed creating metrics receiver")
	_, err = f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, tc)
	require.NoError(t, err, "failed creating traces receiver")
	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, lc)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum && len(tc.AllTraces()) > entriesNum && len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics, %d traces, %d logs in %d minutes", entriesNum,
		len(mc.AllMetrics()), len(tc.AllTraces()), len(lc.AllLogs()), timeoutMinutes)
}
