// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sattributesprocessor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/pipeline/xpipeline"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.uber.org/multierr"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	equal = iota
	regex
	exist
	shouldnotexist
	testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"
	uidRe          = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}"
	startTimeRe    = "^\\\\d{4}-\\\\d{2}-\\\\d{2}T\\\\d{2}%3A\\\\d{2}%3A\\\\d{2}(?:%2E\\\\d+)?[A-Z]?(?:[+.-](?:08%3A\\\\d{2}|\\\\d{2}[A-Z]))?$"
)

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

// TestE2E_ClusterRBAC tests the k8s attributes processor in a k8s cluster with the collector's service account having
// cluster-wide permissions to list/watch namespaces, nodes, pods and replicasets. The config in the test does not
// set filter::namespace, and the telemetrygen image has a latest tag but no digest.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2E_ClusterRBAC(t *testing.T) {
	testDir := filepath.Join("testdata", "e2e", "clusterrbac")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := k8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, k8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	profilesConsumer := new(consumertest.ProfilesSink)
	shutdownSinks := startUpSinks(t, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"), map[string]string{}, "")
	createTeleOpts := &k8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		// `telemetrygen` doesn't support profiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36127
		// TODO: add "profiles" to DataTypes once #36127 is resolved
		DataTypes: []string{"metrics", "logs", "traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := k8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		k8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	wantEntries := 128 // Minimal number of metrics/traces/logs/profiles to wait for.
	waitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)

	tcs := []struct {
		name     string
		dataType pipeline.Signal
		service  string
		attrs    map[string]*expectedValue
	}{
		{
			name:     "traces-job",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-job-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.job.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.job.uid":                  newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "job"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "traces-statefulset",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset-0"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.statefulset.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.statefulset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "traces-deployment",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.deployment.uid":           newExpectedValue(exist, ""),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "traces-daemonset",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.daemonset.name":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.daemonset.uid":            newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "metrics-job",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-job-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.job.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.job.uid":                  newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "job"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "metrics-statefulset",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset-0"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.statefulset.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.statefulset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "metrics-deployment",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.deployment.uid":           newExpectedValue(exist, ""),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "metrics-daemonset",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.daemonset.name":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.daemonset.uid":            newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "logs-job",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-job-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.job.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.job.uid":                  newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "job"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "logs-statefulset",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset-0"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.statefulset.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.statefulset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "logs-deployment",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.deployment.uid":           newExpectedValue(exist, ""),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "logs-daemonset",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.daemonset.name":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.daemonset.uid":            newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "profiles-job",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-job-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.job.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-job"),
				"k8s.job.uid":                  newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "job"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-job"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "profiles-statefulset",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-statefulset-0"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.statefulset.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-statefulset"),
				"k8s.statefulset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-statefulset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "profiles-deployment",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.deployment.uid":           newExpectedValue(exist, ""),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "profiles-daemonset",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.daemonset.name":           newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-daemonset"),
				"k8s.daemonset.uid":            newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-daemonset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.dataType {
			case pipeline.SignalTraces:
				scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
			case pipeline.SignalMetrics:
				scanMetricsForAttributes(t, metricsConsumer, tc.service, tc.attrs)
			case pipeline.SignalLogs:
				scanLogsForAttributes(t, logsConsumer, tc.service, tc.attrs)
			case xpipeline.SignalProfiles:
				scanProfilesForAttributes(t, profilesConsumer, tc.service, tc.attrs)
			default:
				t.Fatalf("unknown data type %s", tc.dataType)
			}
		})
	}
}

// Test with `filter::namespace` set and only role binding to collector's SA. We can't get node and namespace labels/annotations,
// and the telemetrygen image has a digest but no tag.
func TestE2E_NamespacedRBAC(t *testing.T) {
	testDir := filepath.Join("testdata", "e2e", "namespacedrbac")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := k8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)
	nsName := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, k8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", nsName)
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	profilesConsumer := new(consumertest.ProfilesSink)
	shutdownSinks := startUpSinks(t, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"), map[string]string{}, "")
	createTeleOpts := &k8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, nsName),
		// `telemetrygen` doesn't support profiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36127
		// TODO: add "profiles" to DataTypes once #36127 is resolved
		DataTypes: []string{"metrics", "logs", "traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := k8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		k8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	wantEntries := 20 // Minimal number of metrics/traces/logs/profiles to wait for.
	waitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)

	tcs := []struct {
		name     string
		dataType pipeline.Signal
		service  string
		attrs    map[string]*expectedValue
	}{
		{
			name:     "traces-deployment",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, startTimeRe),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, nsName),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
			},
		},
		{
			name:     "metrics-deployment",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, startTimeRe),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, nsName),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
			},
		},
		{
			name:     "logs-deployment",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, startTimeRe),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, nsName),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
			},
		},
		{
			name:     "profiles-deployment",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, startTimeRe),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, nsName),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.dataType {
			case pipeline.SignalTraces:
				scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
			case pipeline.SignalMetrics:
				scanMetricsForAttributes(t, metricsConsumer, tc.service, tc.attrs)
			case pipeline.SignalLogs:
				scanLogsForAttributes(t, logsConsumer, tc.service, tc.attrs)
			case xpipeline.SignalProfiles:
				scanProfilesForAttributes(t, profilesConsumer, tc.service, tc.attrs)
			default:
				t.Fatalf("unknown data type %s", tc.dataType)
			}
		})
	}
}

// Test with `filter::namespace` set, role binding for namespace-scoped objects (pod, replicaset) and clusterrole
// binding for node and namespace objects, and the telemetrygen image has a tag and digest.
func TestE2E_MixRBAC(t *testing.T) {
	testDir := filepath.Join("testdata", "e2e", "mixrbac")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	profilesConsumer := new(consumertest.ProfilesSink)
	shutdownSinks := startUpSinks(t, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)
	defer shutdownSinks()

	var workloadNs, otelNs string
	testID := uuid.NewString()[:8]
	for i, nsManifest := range []string{filepath.Join(testDir, "otelcol-namespace.yaml"), filepath.Join(testDir, "workload-namespace.yaml")} {
		buf, err := os.ReadFile(nsManifest)
		require.NoErrorf(t, err, "failed to read namespace object file %s", nsManifest)
		nsObj, err := k8stest.CreateObject(k8sClient, buf)
		require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsManifest)
		switch i {
		case 0:
			otelNs = nsObj.GetName()
		case 1:
			workloadNs = nsObj.GetName()
		}

		defer func() {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", nsObj.GetName(), "")
		}()
	}

	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"), map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	createTeleOpts := &k8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, otelNs),
		// `telemetrygen` doesn't support profiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36127
		// TODO: add "profiles" to DataTypes once #36127 is resolved
		DataTypes: []string{"metrics", "logs", "traces"},
	}

	telemetryGenObjs, telemetryGenObjInfos := k8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range telemetryGenObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		k8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	wantEntries := 20 // Minimal number of metrics/traces/logs/profiles to wait for.
	waitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)

	tcs := []struct {
		name     string
		dataType pipeline.Signal
		service  string
		attrs    map[string]*expectedValue
	}{
		{
			name:     "traces-deployment",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, workloadNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "0.112.0"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
			},
		},
		{
			name:     "metrics-deployment",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, workloadNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "0.112.0"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
			},
		},
		{
			name:     "logs-deployment",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, workloadNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "0.112.0"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
			},
		},
		{
			name:     "profiles-deployment",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, workloadNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.dataType {
			case pipeline.SignalTraces:
				scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
			case pipeline.SignalMetrics:
				scanMetricsForAttributes(t, metricsConsumer, tc.service, tc.attrs)
			case pipeline.SignalLogs:
				scanLogsForAttributes(t, logsConsumer, tc.service, tc.attrs)
			case xpipeline.SignalProfiles:
				scanProfilesForAttributes(t, profilesConsumer, tc.service, tc.attrs)
			default:
				t.Fatalf("unknown data type %s", tc.dataType)
			}
		})
	}
}

// Test with `filter::namespace` set and only role binding to collector's SA. We can't get node and namespace labels/annotations.
// While `k8s.pod.ip` is not set in `k8sattributes:extract:metadata` and the `pod_association` is not `connection`
// we expect that the `k8s.pod.ip` metadata is not added.
// While `container.image.repo_digests` is not set in `k8sattributes::extract::metadata`, we expect
// that the `container.image.repo_digests` metadata is not added.
// The telemetrygen image has neither a tag nor digest (implicitly latest version)
func TestE2E_NamespacedRBACNoPodIP(t *testing.T) {
	testDir := filepath.Join("testdata", "e2e", "namespaced_rbac_no_pod_ip")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := k8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)
	nsName := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, k8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", nsName)
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	profilesConsumer := new(consumertest.ProfilesSink)
	shutdownSinks := startUpSinks(t, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"), map[string]string{}, "")
	createTeleOpts := &k8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, nsName),
		// `telemetrygen` doesn't support profiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36127
		// TODO: add "profiles" to DataTypes once #36127 is resolved
		DataTypes: []string{"metrics", "logs", "traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := k8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		k8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	wantEntries := 20 // Minimal number of metrics/traces/logs/profiles to wait for.
	waitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)

	tcs := []struct {
		name     string
		dataType pipeline.Signal
		service  string
		attrs    map[string]*expectedValue
	}{
		{
			name:     "traces-deployment",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(shouldnotexist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, startTimeRe),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, nsName),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(shouldnotexist, ""),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
			},
		},
		{
			name:     "metrics-deployment",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(shouldnotexist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, startTimeRe),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, nsName),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(shouldnotexist, ""),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
			},
		},
		{
			name:     "logs-deployment",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(shouldnotexist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, startTimeRe),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, nsName),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(shouldnotexist, ""),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
			},
		},
		{
			name:     "profiles-deployment",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(shouldnotexist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, startTimeRe),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, nsName),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.deployment.uid":           newExpectedValue(regex, uidRe),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(regex, uidRe),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(shouldnotexist, ""),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.dataType {
			case pipeline.SignalTraces:
				scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
			case pipeline.SignalMetrics:
				scanMetricsForAttributes(t, metricsConsumer, tc.service, tc.attrs)
			case pipeline.SignalLogs:
				scanLogsForAttributes(t, logsConsumer, tc.service, tc.attrs)
			case xpipeline.SignalProfiles:
				scanProfilesForAttributes(t, profilesConsumer, tc.service, tc.attrs)
			default:
				t.Fatalf("unknown data type %s", tc.dataType)
			}
		})
	}
}

// TestE2E_ClusterRBACCollectorRestart tests the k8s attributes processor in a k8s cluster with the collector's service account having
// cluster-wide permissions to list/watch namespaces, nodes, pods and replicasets. The config in the test does not
// set filter::namespace, and the telemetrygen image has a latest tag but no digest.
// The test starts the collector after the telemetrygen objects, to verify the initial sync of the k8sattributesproccessor
// captures all available resources in the cluster in a way that ensures the relations between them (e.g. pod <-> replicaset <-> deployment)  are not lost.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2E_ClusterRBACCollectorStartAfterTelemetryGen(t *testing.T) {
	testDir := filepath.Join("testdata", "e2e", "clusterrbac")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := k8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, k8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	profilesConsumer := new(consumertest.ProfilesSink)
	shutdownSinks := startUpSinks(t, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	createTeleOpts := &k8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		// `telemetrygen` doesn't support profiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36127
		// TODO: add "profiles" to DataTypes once #36127 is resolved
		DataTypes: []string{"metrics", "logs", "traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := k8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range telemetryGenObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		k8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	// start the collector after the telemetry gen objects
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"), map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 128 // Minimal number of metrics/traces/logs/profiles to wait for.
	waitForData(t, wantEntries, metricsConsumer, tracesConsumer, logsConsumer, profilesConsumer)

	tcs := []struct {
		name     string
		dataType pipeline.Signal
		service  string
		attrs    map[string]*expectedValue
	}{
		{
			name:     "traces-job",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-job-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.job.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.job.uid":                  newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "job"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "traces-statefulset",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset-0"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.statefulset.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.statefulset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "traces-deployment",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.deployment.uid":           newExpectedValue(exist, ""),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "traces-daemonset",
			dataType: pipeline.SignalTraces,
			service:  "test-traces-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.daemonset.name":           newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.daemonset.uid":            newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "metrics-job",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-job-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.job.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.job.uid":                  newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "job"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-job"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "metrics-statefulset",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset-0"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.statefulset.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.statefulset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-statefulset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "metrics-deployment",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.deployment.uid":           newExpectedValue(exist, ""),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "metrics-daemonset",
			dataType: pipeline.SignalMetrics,
			service:  "test-metrics-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-metrics-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.daemonset.name":           newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.daemonset.uid":            newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-metrics-daemonset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "logs-job",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-job-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.job.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.job.uid":                  newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "job"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-job"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "logs-statefulset",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset-0"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.statefulset.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.statefulset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-statefulset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "logs-deployment",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.deployment.uid":           newExpectedValue(exist, ""),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-logs-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "logs-daemonset",
			dataType: pipeline.SignalLogs,
			service:  "test-logs-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-logs-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.daemonset.name":           newExpectedValue(equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.daemonset.uid":            newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-logs-daemonset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "profiles-job",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-job-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.job.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-job"),
				"k8s.job.uid":                  newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "job"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-job"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "profiles-statefulset",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-statefulset-0"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.statefulset.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-statefulset"),
				"k8s.statefulset.uid":          newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "statefulset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-statefulset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "profiles-deployment",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.deployment.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.deployment.uid":           newExpectedValue(exist, ""),
				"k8s.replicaset.name":          newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-deployment-[a-z0-9]*"),
				"k8s.replicaset.uid":           newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "deployment"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-deployment"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
		{
			name:     "profiles-daemonset",
			dataType: xpipeline.SignalProfiles,
			service:  "test-profiles-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                 newExpectedValue(regex, "telemetrygen-"+testID+"-profiles-daemonset-[a-z0-9]*"),
				"k8s.pod.ip":                   newExpectedValue(exist, ""),
				"k8s.pod.uid":                  newExpectedValue(regex, uidRe),
				"k8s.pod.start_time":           newExpectedValue(exist, ""),
				"k8s.node.name":                newExpectedValue(exist, ""),
				"k8s.namespace.name":           newExpectedValue(equal, testNs),
				"k8s.daemonset.name":           newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-daemonset"),
				"k8s.daemonset.uid":            newExpectedValue(exist, ""),
				"k8s.annotations.workload":     newExpectedValue(equal, "daemonset"),
				"k8s.labels.app":               newExpectedValue(equal, "telemetrygen-"+testID+"-profiles-daemonset"),
				"k8s.container.name":           newExpectedValue(equal, "telemetrygen"),
				"k8s.cluster.uid":              newExpectedValue(regex, uidRe),
				"container.image.name":         newExpectedValue(equal, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen"),
				"container.image.repo_digests": newExpectedValue(regex, "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen@sha256:[0-9a-fA-f]{64}"),
				"container.image.tag":          newExpectedValue(equal, "latest"),
				"container.id":                 newExpectedValue(exist, ""),
				"k8s.node.labels.foo":          newExpectedValue(equal, "too"),
				"k8s.namespace.labels.foons":   newExpectedValue(equal, "barns"),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.dataType {
			case pipeline.SignalTraces:
				scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
			case pipeline.SignalMetrics:
				scanMetricsForAttributes(t, metricsConsumer, tc.service, tc.attrs)
			case pipeline.SignalLogs:
				scanLogsForAttributes(t, logsConsumer, tc.service, tc.attrs)
			case xpipeline.SignalProfiles:
				scanProfilesForAttributes(t, profilesConsumer, tc.service, tc.attrs)
			default:
				t.Fatalf("unknown data type %s", tc.dataType)
			}
		})
	}
}

func scanTracesForAttributes(t *testing.T, ts *consumertest.TracesSink, expectedService string,
	kvs map[string]*expectedValue,
) {
	// Iterate over the received set of traces starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ts.AllTraces()) - 1; i >= 0; i-- {
		traces := ts.AllTraces()[i]
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			resource := traces.ResourceSpans().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.True(t, exist, "span do not has 'service.name' attribute in resource")
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
	kvs map[string]*expectedValue,
) {
	// Iterate over the received set of metrics starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ms.AllMetrics()) - 1; i >= 0; i-- {
		metrics := ms.AllMetrics()[i]
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			resource := metrics.ResourceMetrics().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.True(t, exist, "metric do not has 'service.name' attribute in resource")
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
	kvs map[string]*expectedValue,
) {
	// Iterate over the received set of logs starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ls.AllLogs()) - 1; i >= 0; i-- {
		logs := ls.AllLogs()[i]
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			resource := logs.ResourceLogs().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.True(t, exist, "log do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no logs found for service %s", expectedService)
}

func scanProfilesForAttributes(t *testing.T, ps *consumertest.ProfilesSink, expectedService string,
	kvs map[string]*expectedValue,
) {
	// `telemetrygen` doesn't support profiles
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36127
	// TODO: Remove `t.Skip()` once #36127 is resolved
	t.Skip("Skip profiles test")

	// Iterate over the received set of profiles starting from the most recent entries due to a bug in the processor:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18892
	// TODO: Remove the reverse loop once it's fixed. All the metrics should be properly annotated.
	for i := len(ps.AllProfiles()) - 1; i >= 0; i-- {
		profiles := ps.AllProfiles()[i]
		for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
			resource := profiles.ResourceProfiles().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.True(t, exist, "profile do not has 'service.name' attribute in resource")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no profiles found for service %s", expectedService)
}

func resourceHasAttributes(resource pcommon.Resource, kvs map[string]*expectedValue) error {
	foundAttrs := make(map[string]bool)
	shouldNotFoundAttrs := make(map[string]bool)
	for k, v := range kvs {
		if v.mode != shouldnotexist {
			foundAttrs[k] = false
			continue
		}
		shouldNotFoundAttrs[k] = false
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
				case shouldnotexist:
					shouldNotFoundAttrs[k] = true
				}
			}
			return true
		},
	)

	var err error
	for k, v := range shouldNotFoundAttrs {
		if v {
			err = multierr.Append(err, fmt.Errorf("%v attribute should not be added", k))
		}
	}
	for k, v := range foundAttrs {
		if !v {
			err = multierr.Append(err, fmt.Errorf("%v attribute not found", k))
		}
	}
	return err
}

func startUpSinks(t *testing.T, mc *consumertest.MetricsSink, tc *consumertest.TracesSink, lc *consumertest.LogsSink, pc *consumertest.ProfilesSink) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.HTTP = nil
	cfg.GRPC.NetAddr.Endpoint = "0.0.0.0:4317"

	_, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, mc)
	require.NoError(t, err, "failed creating metrics receiver")
	_, err = f.CreateTraces(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, tc)
	require.NoError(t, err, "failed creating traces receiver")
	_, err = f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, lc)
	require.NoError(t, err, "failed creating logs receiver")
	rcvr, err := f.(xreceiver.Factory).CreateProfiles(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, pc)
	require.NoError(t, err, "failed creating profiles receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	return func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}
}

func waitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink, tc *consumertest.TracesSink, lc *consumertest.LogsSink, pc *consumertest.ProfilesSink) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		// `telemetrygen` doesn't support profiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36127
		// TODO: assert `len(pc.AllProfiles()) > entriesNum` once #36127 is resolved
		return len(mc.AllMetrics()) > entriesNum && len(tc.AllTraces()) > entriesNum && len(lc.AllLogs()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics, %d traces, %d logs, %d profiles in %d minutes", entriesNum,
		len(mc.AllMetrics()), len(tc.AllTraces()), len(lc.AllLogs()), len(pc.AllProfiles()), timeoutMinutes)
}
