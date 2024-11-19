// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sclusterreceiver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	expectedFileClusterScoped   = "./testdata/e2e/cluster-scoped/expected.yaml"
	expectedFileNamespaceScoped = "./testdata/e2e/namespace-scoped/expected.yaml"

	testObjectsDirClusterScoped   = "./testdata/e2e/cluster-scoped/testobjects"
	testObjectsDirNamespaceScoped = "./testdata/e2e/namespace-scoped/testobjects"
	testKubeConfig                = "/tmp/kube-config-otelcol-e2e-testing"
)

// TestE2EClusterScoped tests the k8s cluster receiver with a real k8s cluster.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2EClusterScoped(t *testing.T) {
	var expected pmetric.Metrics
	expected, err := golden.ReadMetrics(expectedFileClusterScoped)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	// k8s test objs
	testObjs, err := k8stest.CreateObjects(k8sClient, testObjectsDirClusterScoped)
	require.NoErrorf(t, err, "failed to create objects")

	t.Cleanup(func() {
		require.NoErrorf(t, k8stest.DeleteObjects(k8sClient, testObjs), "failed to delete objects")
	})

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "cluster-scoped", "collector"))

	t.Cleanup(func() {
		for _, obj := range append(collectorObjs) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	})

	wantEntries := 10 // Minimal number of metrics to wait for.
	// the commented line below writes the received list of metrics to the expected.yaml
	// golden.WriteMetrics(t, expectedFileClusterScoped, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])
	waitForData(t, wantEntries, metricsConsumer)

	require.NoError(t, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(
			"k8s.container.cpu_request",
			"k8s.container.memory_limit",
			"k8s.container.memory_request",
			"k8s.container.restarts",
			"k8s.cronjob.active_jobs",
			"k8s.deployment.available",
			"k8s.deployment.desired",
			"k8s.job.active_pods",
			"k8s.job.desired_successful_pods",
			"k8s.job.failed_pods",
			"k8s.job.max_parallel_pods",
			"k8s.hpa.current_replicas",
			"k8s.job.successful_pods"),
		pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.name", containerImageShorten),
		pmetrictest.ChangeResourceAttributeValue("container.image.tag", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.cronjob.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.hpa.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.job.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.job.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.statefulset.uid", replaceWithStar),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	),
	)
}

// TestE2ENamespaceScoped tests the k8s cluster receiver with a real k8s cluster.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2ENamespaceScoped(t *testing.T) {

	var expected pmetric.Metrics
	expected, err := golden.ReadMetrics(expectedFileNamespaceScoped)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	// k8s test objs
	testObjs, err := k8stest.CreateObjects(k8sClient, testObjectsDirNamespaceScoped)
	require.NoErrorf(t, err, "failed to create objects")

	t.Cleanup(func() {
		require.NoErrorf(t, k8stest.DeleteObjects(k8sClient, testObjs), "failed to delete objects")
	})

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "namespace-scoped", "collector"))

	t.Cleanup(func() {
		for _, obj := range append(collectorObjs) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	})

	wantEntries := 10 // Minimal number of metrics to wait for.
	// the commented line below writes the received list of metrics to the expected.yaml
	// golden.WriteMetrics(t, expectedFileNamespaceScoped, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])
	waitForData(t, wantEntries, metricsConsumer)

	require.NoError(t, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(
			"k8s.container.cpu_request",
			"k8s.container.memory_limit",
			"k8s.container.memory_request",
			"k8s.container.restarts",
			"k8s.cronjob.active_jobs",
			"k8s.deployment.available",
			"k8s.deployment.desired",
			"k8s.job.active_pods",
			"k8s.job.desired_successful_pods",
			"k8s.job.failed_pods",
			"k8s.job.max_parallel_pods",
			"k8s.hpa.current_replicas",
			"k8s.job.successful_pods"),
		pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.name", containerImageShorten),
		pmetrictest.ChangeResourceAttributeValue("container.image.tag", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.cronjob.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.hpa.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.job.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.job.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.statefulset.uid", replaceWithStar),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	),
	)
}

func shortenNames(value string) string {
	if strings.HasPrefix(value, "coredns") {
		return "coredns"
	}
	if strings.HasPrefix(value, "kindnet") {
		return "kindnet"
	}
	if strings.HasPrefix(value, "kube-apiserver") {
		return "kube-apiserver"
	}
	if strings.HasPrefix(value, "kube-proxy") {
		return "kube-proxy"
	}
	if strings.HasPrefix(value, "kube-scheduler") {
		return "kube-scheduler"
	}
	if strings.HasPrefix(value, "kube-controller-manager") {
		return "kube-controller-manager"
	}
	if strings.HasPrefix(value, "local-path-provisioner") {
		return "local-path-provisioner"
	}
	if strings.HasPrefix(value, "otelcol") {
		return "otelcol"
	}
	if strings.HasPrefix(value, "test-k8scluster-receiver-cronjob") {
		return "test-k8scluster-receiver-cronjob"
	}
	if strings.HasPrefix(value, "test-k8scluster-receiver-job") {
		return "test-k8scluster-receiver-job"
	}
	return value
}

func replaceWithStar(_ string) string { return "*" }

func containerImageShorten(value string) string {
	// Extracts the image name by removing the repository prefix.
	// Also removes any architecture identifier suffix, if present, by applying shortenNames.
	return shortenNames(value[(strings.LastIndex(value, "/") + 1):])
}

func startUpSink(t *testing.T, mc *consumertest.MetricsSink) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.HTTP = nil
	cfg.GRPC.NetAddr.Endpoint = "0.0.0.0:4317"

	rcvr, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(), cfg, mc)
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
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
