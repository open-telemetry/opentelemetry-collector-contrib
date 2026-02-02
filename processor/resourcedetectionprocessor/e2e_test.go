// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package resourcedetectionprocessor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "env", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "system", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10 // Minimal number of metrics to wait for.
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EDockerDetector validates docker metadata detection by running the Collector inside a
// Docker container and verifying that docker-specific resource attributes are reported.
func TestE2EDockerDetector(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Docker detector e2e test is currently supported on linux runners only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker binary not available: %v", err)
	}
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skipf("docker socket unavailable: %v", err)
	}

	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "docker", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	containerName := fmt.Sprintf("otelcol-docker-%s", strings.ToLower(uuid.NewString()[:8]))
	runCollectorInDocker(t, containerName)

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

	containerResourceName := "/" + containerName
	setResourceAttributeValue(expected, "container.name", containerResourceName)

	allMetrics := metricsConsumer.AllMetrics()
	newMetrics := allMetrics[startEntries:]
	var actual pmetric.Metrics
	found := false
	for _, m := range newMetrics {
		var metricsWithContainer pmetric.Metrics
		metricsWithContainer, err = filterMetricsByResourceAttribute(m, "container.name", containerResourceName)
		if err == nil {
			actual = metricsWithContainer
			found = true
			break
		}
	}
	require.Truef(t, found, "resource with container.name=%s not found in new metric batches", containerName)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, actual,
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
			pmetrictest.ChangeResourceAttributeValue("container.image.name", replaceWithStar),
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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "eks", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "gcp", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "heroku", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2ELambdaDetector tests the AWS Lambda detector by setting Lambda environment variables
// and verifying that the resource attributes are correctly detected and attached to metrics.
func TestE2ELambdaDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "lambda", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "lambda", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

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
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "nova", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "upcloud", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "vultr", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "ec2", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EAlibabaECSDetector tests the Alibana Cloud ECS detector by verifying that ECS metadata
// (like cloud.provider, cloud.region, host.id, etc.) is correctly detected and attached to metrics.
func TestE2EAlibabaECSDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "alibaba_ecs", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "alibaba_ecs", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "scaleway", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EDigitalOceanDetector tests the DigitalOcean detector by deploying a metadata-server
// sidecar that simulates the DigitalOcean metadata service and verifying that the resource
// attributes are correctly detected and attached to metrics.
func TestE2EDigitalOceanDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "digitalocean", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "digitalocean", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "azure", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EK8sNodeDetector tests the k8snode detector by querying the K8s API server
// to retrieve node metadata and verifying that the k8s.node.name and k8s.node.uid
// resource attributes are correctly detected and attached to metrics.
func TestE2EK8sNodeDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "k8snode", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "k8snode", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		metrics := metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]

		// Verify that k8snode detector populated the dynamic attributes (not empty)
		require.Greater(tt, metrics.ResourceMetrics().Len(), 0, "expected at least one resource metric")
		resourceAttrs := metrics.ResourceMetrics().At(0).Resource().Attributes()

		nodeName, found := resourceAttrs.Get("k8s.node.name")
		require.True(tt, found, "k8s.node.name attribute should be present")
		require.NotEmpty(tt, nodeName.Str(), "k8s.node.name should not be empty")

		nodeUID, found := resourceAttrs.Get("k8s.node.uid")
		require.True(tt, found, "k8s.node.uid attribute should be present")
		require.NotEmpty(tt, nodeUID.Str(), "k8s.node.uid should not be empty")

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

			pmetrictest.ChangeResourceAttributeValue("k8s.node.name", replaceWithStar),
			pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", replaceWithStar),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EAKSDetector tests the AKS detector by deploying a metadata-server
// sidecar that simulates the Azure IMDS and verifying that the AKS-specific
// resource attributes (cloud.provider, cloud.platform, k8s.cluster.name) are
// correctly detected and attached to metrics.
func TestE2EAKSDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "aks", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "aks", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EOpenShiftDetector tests the OpenShift detector by deploying a metadata-server
// sidecar that simulates the OpenShift API and verifying that the resource attributes
// are correctly detected and attached to metrics.
func TestE2EOpenShiftDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "openshift", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "openshift", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EDynatraceDetector tests the Dynatrace detector by mounting a mock
// dt_host_metadata.properties file and verifying that the resource attributes
// are correctly detected and attached to metrics.
func TestE2EDynatraceDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "dynatrace", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "dynatrace", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EAkamaiDetector tests the Akamai detector by deploying a metadata-server
// sidecar that simulates the Akamai/Linode IMDS (at 169.254.169.254) and verifying
// that the resource attributes are correctly detected and attached to metrics.
func TestE2EAkamaiDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "akamai", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "akamai", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EElasticBeanstalkDetector tests the Elastic Beanstalk detector by mounting a mock
// environment.conf file and verifying that the resource attributes are correctly detected
// and attached to metrics.
func TestE2EElasticBeanstalkDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "elasticbeanstalk", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "elasticbeanstalk", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EHetznerDetector tests the Hetzner detector by deploying a metadata-server
// sidecar that simulates the Hetzner IMDS and verifying that the resource attributes
// are correctly detected and attached to metrics.
func TestE2EHetznerDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "hetzner", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "hetzner", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EECSDetector tests the ECS detector by deploying a metadata-server
// sidecar that simulates the ECS Task Metadata Endpoint V4 and verifying that
// the resource attributes are correctly detected and attached to metrics.
func TestE2EECSDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "ecs", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "ecs", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

// TestE2EKubeadmDetector tests the kubeadm detector by creating the kubeadm-config ConfigMap
// in kube-system namespace and verifying that the k8s.cluster.name and k8s.cluster.uid
// resource attributes are correctly detected and attached to metrics.
func TestE2EKubeadmDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "kubeadm", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "kubeadm", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

	// Uncomment to regenerate golden file
	// golden.WriteMetrics(t, expectedFile+".actual", metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1])

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		metrics := metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]

		// Verify that kubeadm detector populated the dynamic attribute (not empty)
		require.Greater(tt, metrics.ResourceMetrics().Len(), 0, "expected at least one resource metric")
		resourceAttrs := metrics.ResourceMetrics().At(0).Resource().Attributes()

		clusterName, found := resourceAttrs.Get("k8s.cluster.name")
		require.True(tt, found, "k8s.cluster.name attribute should be present")
		require.NotEmpty(tt, clusterName.Str(), "k8s.cluster.name should not be empty")

		clusterUID, found := resourceAttrs.Get("k8s.cluster.uid")
		require.True(tt, found, "k8s.cluster.uid attribute should be present")
		require.NotEmpty(tt, clusterUID.Str(), "k8s.cluster.uid should not be empty")

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

			pmetrictest.ChangeResourceAttributeValue("k8s.cluster.uid", replaceWithStar),
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

// TestE2EOracleCloudDetector tests the Oracle Cloud detector by deploying a metadata-server
// sidecar that simulates the Oracle Cloud IMDS and verifying that the resource attributes
// are correctly detected and attached to metrics.
func TestE2EOracleCloudDetector(t *testing.T) {
	var expected pmetric.Metrics
	expectedFile := filepath.Join("testdata", "e2e", "oraclecloud", "expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
	require.NoError(t, err)

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSink := startUpSink(t, metricsConsumer)
	defer shutdownSink()
	startEntries := len(metricsConsumer.AllMetrics())

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(".", "testdata", "e2e", "oraclecloud", "collector"), map[string]string{}, "")

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 10
	waitForData(t, metricsConsumer, startEntries, wantEntries)

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

func waitForData(t *testing.T, mc *consumertest.MetricsSink, startEntries, entriesNum int) {
	timeoutMinutes := 3
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) >= startEntries+entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d new entries, received %d metrics total (starting from %d) in %d minutes",
		entriesNum, len(mc.AllMetrics()), startEntries, timeoutMinutes)
}

func runCollectorInDocker(t *testing.T, containerName string) {
	t.Helper()

	confDir := filepath.Join("testdata", "e2e", "docker")
	absConfDir, err := filepath.Abs(confDir)
	require.NoError(t, err)

	groupArgs := dockerSocketGroupArgs(t)

	args := []string{
		"run",
		"-d",
		"--name", containerName,
		"--add-host", "host.docker.internal:host-gateway",
	}
	args = append(args, groupArgs...)
	args = append(args,
		"-v", fmt.Sprintf("%s:/conf:ro", absConfDir),
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"otelcontribcol:latest",
		"--config=/conf/config.yaml",
	)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "failed to start docker collector: %s", output)

	t.Cleanup(func() {
		if t.Failed() {
			if logs, logErr := exec.Command("docker", "logs", containerName).CombinedOutput(); logErr == nil {
				t.Logf("docker logs for %s:\n%s", containerName, logs)
			} else {
				t.Logf("failed to fetch docker logs for %s: %v", containerName, logErr)
			}
		}
		stopDockerContainer(t, containerName)
	})
}

func stopDockerContainer(t *testing.T, containerName string) {
	t.Helper()
	cmd := exec.Command("docker", "rm", "-f", containerName)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("failed to remove docker container %s: %v (%s)", containerName, err, output)
	}
}

func setResourceAttributeValue(metrics pmetric.Metrics, key, value string) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		attrs := rms.At(i).Resource().Attributes()
		if _, ok := attrs.Get(key); ok {
			attrs.PutStr(key, value)
		}
	}
}

func filterMetricsByResourceAttribute(metrics pmetric.Metrics, key, value string) (pmetric.Metrics, error) {
	filtered := pmetric.NewMetrics()
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		attrs := rms.At(i).Resource().Attributes()
		if attr, ok := attrs.Get(key); ok && attr.AsString() == value {
			rms.At(i).CopyTo(filtered.ResourceMetrics().AppendEmpty())
		}
	}
	if filtered.ResourceMetrics().Len() == 0 {
		return filtered, fmt.Errorf("resource with %s=%s not found", key, value)
	}
	return filtered, nil
}

func dockerSocketGroupArgs(t *testing.T) []string {
	info, err := os.Stat("/var/run/docker.sock")
	require.NoError(t, err)

	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok, "unexpected stat type for docker socket")

	return []string{"--group-add", strconv.FormatUint(uint64(stat.Gid), 10)}
}
