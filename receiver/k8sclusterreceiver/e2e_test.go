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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const testKubeConfig = "/tmp/kube-config-otelcol-e2e-testing"

// TestE2E tests the k8s cluster receiver with a real k8s cluster.
// The test requires a prebuilt otelcontribcol image uploaded to a kind k8s cluster defined in
// `/tmp/kube-config-otelcol-e2e-testing`. Run the following command prior to running the test locally:
//
//	kind create cluster --kubeconfig=/tmp/kube-config-otelcol-e2e-testing
//	make docker-otelcontribcol
//	KUBECONFIG=/tmp/kube-config-otelcol-e2e-testing kind load docker-image otelcontribcol:latest
func TestE2E(t *testing.T) {

	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, "")

	defer func() {
		for _, obj := range append(collectorObjs) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10 // Minimal number of metrics to wait for.
	waitForData(t, wantEntries, metricsConsumer)

	replaceWithStar := func(string) string { return "*" }
	shortenNames := func(value string) string {
		if strings.HasPrefix(value, "kube-proxy") {
			return "kube-proxy"
		}
		if strings.HasPrefix(value, "local-path-provisioner") {
			return "local-path-provisioner"
		}
		if strings.HasPrefix(value, "kindnet") {
			return "kindnet"
		}
		if strings.HasPrefix(value, "coredns") {
			return "coredns"
		}
		if strings.HasPrefix(value, "otelcol") {
			return "otelcol"
		}
		return value
	}
	containerImageShorten := func(value string) string {
		return value[(strings.LastIndex(value, "/") + 1):]
	}
	require.NoError(t, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues("k8s.deployment.desired", "k8s.deployment.available", "k8s.container.restarts", "k8s.container.cpu_request", "k8s.container.memory_request", "k8s.container.memory_limit"),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.tag", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.name", containerImageShorten),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	),
	)
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
