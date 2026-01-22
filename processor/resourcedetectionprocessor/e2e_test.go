// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package resourcedetectionprocessor

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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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
	require.NotNil(t, val, "Expected a default value to be set for %T", val)
	return val
}

// TestE2EEnvDetector tests the env detector by setting environment variables and verifying
// that the resource attributes are correctly detected and attached to metrics.
func TestE2EEnvDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "env", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "env", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2ESystemDetector tests the system detector by verifying that system metadata
// (like host.name, os.type, etc.) is correctly detected and attached to metrics.
func TestE2ESystemDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "system", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "system", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10 // Minimal number of metrics to wait for.
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		// For system detector, we need to ignore some values that are host-specific
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),

			pmetrictest.ChangeResourceAttributeValue("host.name", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("host.arch", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("os.version", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("os.description", replaceWithStar),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EEKSDetector validates the eks detector using mocked Kubernetes and EC2 metadata endpoints.
func TestE2EEKSDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "eks", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "eks", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EGCPDetector validates that the gcp detector can populate Compute Engine
// metadata by pointing the metadata client to a fake metadata server.
func TestE2EGCPDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "gcp", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "gcp", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EHerokuDetector tests the Heroku detector by setting Heroku environment variables
// and verifying that the resource attributes are correctly detected and attached to metrics.
func TestE2EHerokuDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "heroku", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "heroku", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EConsulDetector tests the consul detector by deploying a collector with a Consul agent sidecar
// and verifying that consul metadata (datacenter, node ID, hostname, and custom meta labels)
// is correctly detected and attached to metrics.
func TestE2EConsulDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "consul", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]

	// Deploy collector with Consul agent sidecar
	// CreateCollectorObjects waits for pods to be ready before returning
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "consul", "collector"), map[string]string{}, "")
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete collector object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		metrics := metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]

		// Verify that consul detector populated the dynamic attributes (not empty)
		require.Greater(tt, metrics.ResourceMetrics().Len(), 0, "expected at least one resource metric")
		resourceAttrs := metrics.ResourceMetrics().At(0).Resource().Attributes()

		hostName, found := resourceAttrs.Get("host.name")
		require.True(tt, found, "host.name attribute should be present")
		require.NotEmpty(tt, hostName.Str(), "host.name should not be empty")

		hostID, found := resourceAttrs.Get("host.id")
		require.True(tt, found, "host.id attribute should be present")
		require.NotEmpty(tt, hostID.Str(), "host.id should not be empty")

		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metrics,
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),

			pmetrictest.ChangeResourceAttributeValue("host.name", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("host.id", replaceWithStar),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EOpenstackNovaDetector tests the OpenStack Nova detector by deploying a metadata-server
// sidecar that simulates the OpenStack metadata service and verifying that the resource attributes
// are correctly detected and attached to metrics.
func TestE2EOpenstackNovaDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "nova", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "nova", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EUpcloudDetector tests the UpCloud detector by deploying a metadata-server
// sidecar that simulates the UpCloud IMDS and verifying that the resource attributes
// are correctly detected and attached to metrics.
func TestE2EUpcloudDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "upcloud", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "upcloud", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EVultrDetector tests the Vultr detector by deploying a metadata-server
// sidecar that simulates the Vultr IMDS and verifying that the resource attributes
// are correctly detected and attached to metrics.
func TestE2EVultrDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "vultr", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "vultr", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EEC2Detector tests the EC2 detector by verifying that EC2 metadata
// (like cloud.provider, cloud.region, host.id, etc.) is correctly detected and attached to metrics.
func TestE2EEC2Detector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "ec2", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "ec2", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EScalewayDetector tests the Scaleway detector by deploying a metadata-server
// sidecar that simulates the Scaleway IMDS (at 169.254.42.42) and verifying that
// the resource attributes are correctly detected and attached to metrics.
func TestE2EScalewayDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "scaleway", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "scaleway", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EAzureDetector tests the Azure detector by deploying a metadata-server
// sidecar that simulates the Azure IMDS and verifying that the resource attributes
// are correctly detected and attached to metrics.
func TestE2EAzureDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "azure", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "azure", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, wantEntries, metricsConsumer)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreScopeVersion(),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("system.cpu.time"),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

func replaceWithStar(_ string) string {
	return "*"
}

func startUpSink(t *testing.T, mc *consumertest.MetricsSink) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)
	getOrInsertDefault(t, &cfg.GRPC).NetAddr.Endpoint = "0.0.0.0:4317"
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
		"failed to receive %d entries, received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
}
