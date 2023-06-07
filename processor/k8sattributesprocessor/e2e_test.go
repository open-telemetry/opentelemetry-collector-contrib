// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package k8sattributesprocessor

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"
)

const testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"

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
	k8stest.WaitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer)

	tcs := []struct {
		name     string
		dataType component.DataType
		service  string
		attrs    map[string]*k8stest.ExpectedValue
	}{
		{
			name:     "traces-job",
			dataType: component.DataTypeTraces,
			service:  "test-traces-job",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-traces-job-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.job.name":             k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.job.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "job"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "traces-statefulset",
			dataType: component.DataTypeTraces,
			service:  "test-traces-statefulset",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-statefulset-0"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.statefulset.name":     k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.statefulset.uid":      k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "statefulset"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "traces-deployment",
			dataType: component.DataTypeTraces,
			service:  "test-traces-deployment",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.deployment.name":      k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.deployment.uid":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.replicaset.name":      k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "deployment"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "traces-daemonset",
			dataType: component.DataTypeTraces,
			service:  "test-traces-daemonset",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.daemonset.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.daemonset.uid":        k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "daemonset"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "metrics-job",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-job",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-metrics-job-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.job.name":             k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.job.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "job"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "metrics-statefulset",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-statefulset",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-statefulset-0"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.statefulset.name":     k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.statefulset.uid":      k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "statefulset"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "metrics-deployment",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-deployment",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.deployment.name":      k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.deployment.uid":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.replicaset.name":      k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "deployment"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "metrics-daemonset",
			dataType: component.DataTypeMetrics,
			service:  "test-metrics-daemonset",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-metrics-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.daemonset.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.daemonset.uid":        k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "daemonset"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "logs-job",
			dataType: component.DataTypeLogs,
			service:  "test-logs-job",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-logs-job-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.job.name":             k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.job.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "job"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "logs-statefulset",
			dataType: component.DataTypeLogs,
			service:  "test-logs-statefulset",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-statefulset-0"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.statefulset.name":     k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.statefulset.uid":      k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "statefulset"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "logs-deployment",
			dataType: component.DataTypeLogs,
			service:  "test-logs-deployment",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.deployment.name":      k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.deployment.uid":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.replicaset.name":      k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "deployment"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
		{
			name:     "logs-daemonset",
			dataType: component.DataTypeLogs,
			service:  "test-logs-daemonset",
			attrs: map[string]*k8stest.ExpectedValue{
				"k8s.pod.name":             k8stest.NewExpectedValue(k8stest.Regex, "telemetrygen-"+testID+"-logs-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":               k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.uid":              k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.pod.start_time":       k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.node.name":            k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.namespace.name":       k8stest.NewExpectedValue(k8stest.Equal, "default"),
				"k8s.daemonset.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.daemonset.uid":        k8stest.NewExpectedValue(k8stest.Exist, ""),
				"k8s.annotations.workload": k8stest.NewExpectedValue(k8stest.Equal, "daemonset"),
				"k8s.labels.app":           k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.container.name":       k8stest.NewExpectedValue(k8stest.Equal, "telemetrygen"),
				"container.image.name":     k8stest.NewExpectedValue(k8stest.Equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.tag":      k8stest.NewExpectedValue(k8stest.Equal, "latest"),
				"container.id":             k8stest.NewExpectedValue(k8stest.Exist, ""),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.dataType {
			case component.DataTypeTraces:
				k8stest.ScanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
			case component.DataTypeMetrics:
				k8stest.ScanMetricsForAttributes(t, metricsConsumer, tc.service, tc.attrs)
			case component.DataTypeLogs:
				k8stest.ScanLogsForAttributes(t, logsConsumer, tc.service, tc.attrs)
			default:
				t.Fatalf("unknown data type %s", tc.dataType)
			}
		})
	}
}
