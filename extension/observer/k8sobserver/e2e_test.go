// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sobserver

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	testKubeConfig                = "/tmp/kube-config-otelcol-e2e-testing"
	namespacedCollectorObjectsDir = "./testdata/e2e/namespaced/collector/"
	namespacedExpectedDir         = "./testdata/e2e/namespaced/expected/"
)

func TestE2ENamespaced(t *testing.T) {
	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]

	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	cfg.HTTP = nil
	cfg.GRPC.NetAddr.Endpoint = "0.0.0.0:4317"
	metricsConsumer := new(consumertest.MetricsSink)

	rcvr, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), cfg, metricsConsumer)
	require.NoError(t, err, "failed creating logs receiver")
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

	expectedFile := filepath.Join(namespacedExpectedDir, "metrics.yaml")

	require.Eventuallyf(t, func() bool {
		return len(metricsConsumer.AllMetrics()) > 0
	}, 1*time.Minute, 1*time.Second,
		"Timeout: failed to receive logs in 1 minute")

	// golden.WriteMetrics(t, expectedFile, metricsConsumer.AllMetrics()[0])

	var expected pmetric.Metrics
	expected, err = golden.ReadMetrics(expectedFile)
	require.NoErrorf(t, err, "Error reading metrics from %s", expectedFile)

	require.NoErrorf(t, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[0],
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(
			"otelcol_exporter_queue_capacity",
			"otelcol_exporter_queue_size",
			"otelcol_process_cpu_seconds_total",
			"otelcol_process_memory_rss_bytes",
			"otelcol_process_runtime_heap_alloc_bytes",
			"otelcol_process_runtime_total_alloc_bytes_total",
			"otelcol_process_runtime_total_sys_memory_bytes",
			"otelcol_process_uptime_seconds_total",
			"promhttp_metric_handler_errors_total",
			"scrape_duration_seconds",
			"scrape_samples_scraped",
			"scrape_samples_post_metric_relabeling",
			"scrape_series_added"),
		pmetrictest.ChangeResourceAttributeValue("container.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("service.instance.id", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("service.version", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("server.address", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("net.host.name", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.tag", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.cronjob.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.hpa.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.job.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", replaceWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", shortenNames),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", replaceWithStar),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder()),
		"Received metrics did not match metrics in file %s", expectedFile,
	)
}

func replaceWithStar(_ string) string { return "*" }

func shortenNames(value string) string {
	if strings.HasPrefix(value, "otelcol") {
		return "otelcol"
	}
	return value
}
