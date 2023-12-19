// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package k8sattributesprocessor

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"
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

// TestE2E tests the k8s attributes processor with a real k8s cluster.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
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

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	shutdownSinks := startUpSinks(t, metricsConsumer, tracesConsumer, logsConsumer)
	defer shutdownSinks()

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

	wantEntries := 128 // Minimal number of metrics/traces/logs to wait for.
	waitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer)

	tcs := []struct {
		name     string
		dataType component.DataType
		service  string
		attrs    map[string]*expectedValue
	}{
		{
			name:     "traces-job",
			dataType: component.DataTypeTraces,
			service:  "test-traces-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-traces-job-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.job.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.job.uid":              newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "job"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
			},
		},
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
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
			},
		},
		{
			name:     "traces-deployment",
			dataType: component.DataTypeTraces,
			service:  "test-traces-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.deployment.name":      newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.deployment.uid":       newExpectedValue(exist, ""),
				"k8s.replicaset.name":      newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "deployment"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "traces-daemonset",
			dataType: component.DataTypeTraces,
			service:  "test-traces-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.daemonset.name":       newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.daemonset.uid":        newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
			},
		},
		{
			name:     "metrics-job",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-job-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.job.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.job.uid":              newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "job"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
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
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
			},
		},
		{
			name:     "metrics-deployment",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.deployment.name":      newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.deployment.uid":       newExpectedValue(exist, ""),
				"k8s.replicaset.name":      newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "deployment"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
			},
		},
		{
			name:     "metrics-daemonset",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.daemonset.name":       newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.daemonset.uid":        newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
			},
		},
		{
			name:     "logs-job",
			dataType: component.DataTypeLogs,
			service:  "test-logs-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-logs-job-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.job.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.job.uid":              newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "job"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
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
		{
			name:     "logs-deployment",
			dataType: component.DataTypeLogs,
			service:  "test-logs-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.deployment.name":      newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.deployment.uid":       newExpectedValue(exist, ""),
				"k8s.replicaset.name":      newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "deployment"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
			},
		},
		{
			name:     "logs-daemonset",
			dataType: component.DataTypeLogs,
			service:  "test-logs-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-logs-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               newExpectedValue(exist, ""),
				"k8s.pod.uid":              newExpectedValue(exist, ""),
				"k8s.pod.start_time":       newExpectedValue(exist, ""),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.namespace.name":       newExpectedValue(equal, "default"),
				"k8s.daemonset.name":       newExpectedValue(equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.daemonset.uid":        newExpectedValue(exist, ""),
				"k8s.annotations.workload": newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.container.name":       newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":          newExpectedValue(exist, ""),
				"container.image.name":     newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      newExpectedValue(equal, "latest"),
				"container.id":             newExpectedValue(exist, ""),
				"k8s.node.labels.foo":      newExpectedValue(equal, "too"),
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
	// Iterate over the received set of traces starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ts.AllTraces()) - 1; i >= 0; i-- {
		traces := ts.AllTraces()[i]
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			resource := traces.ResourceSpans().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "span do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no spans found for service %s", expectedService)
}

func scanMetricsForAttributes(t *testing.T, ms *consumertest.MetricsSink, expectedService string,
	kvs map[string]*expectedValue) {
	// Iterate over the received set of metrics starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ms.AllMetrics()) - 1; i >= 0; i-- {
		metrics := ms.AllMetrics()[i]
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			resource := metrics.ResourceMetrics().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "metric do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no metric found for service %s", expectedService)
}

func scanLogsForAttributes(t *testing.T, ls *consumertest.LogsSink, expectedService string,
	kvs map[string]*expectedValue) {
	// Iterate over the received set of logs starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ls.AllLogs()) - 1; i >= 0; i-- {
		logs := ls.AllLogs()[i]
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			resource := logs.ResourceLogs().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "log do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no logs found for service %s", expectedService)
}

func resourceHasAttributes(resource pcommon.Resource, kvs map[string]*expectedValue) error {
	foundAttrs := make(map[string]bool)
	for k := range kvs {
		foundAttrs[k] = false
	}

	resource.Attributes().Range(
		func(k string, v pcommon.Value) bool {
			if val, ok := kvs[k]; ok {
				switch val.mode {
				case equal:
					if val.value == v.AsString() {
						foundAttrs[k] = true
					}
				case regex:
					matched, _ := regexp.MatchString(val.value, v.AsString())
					if matched {
						foundAttrs[k] = true
					}
				case exist:
					foundAttrs[k] = true
				}

			}
			return true
		},
	)

	var err error
	for k, v := range foundAttrs {
		if !v {
			err = multierr.Append(err, fmt.Errorf("%v attribute not found", k))
		}
	}
	return err
}

func startUpSinks(t *testing.T, mc *consumertest.MetricsSink, tc *consumertest.TracesSink, lc *consumertest.LogsSink) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	_, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, err, "failed creating metrics receiver")
	_, err = f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, tc)
	require.NoError(t, err, "failed creating traces receiver")
	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, lc)
	require.NoError(t, err, "failed creating logs receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	return func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}
}

func waitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink, tc *consumertest.TracesSink, lc *consumertest.LogsSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum && len(tc.AllTraces()) > entriesNum && len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics, %d traces, %d logs in %d minutes", entriesNum,
		len(mc.AllMetrics()), len(tc.AllTraces()), len(lc.AllLogs()), timeoutMinutes)
}
