// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sobserver

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
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
	var expected pmetric.Metrics
	expected, err = golden.ReadMetrics(expectedFile)
	require.NoErrorf(t, err, "Error reading metrics from %s", expectedFile)

	require.Eventuallyf(t, func() bool {
		return len(metricsConsumer.AllMetrics()) > 0
	}, 1*time.Minute, 1*time.Second,
		"Timeout: failed to receive logs in 1 minute")

	// golden.WriteLogs(t, expectedFile, logsConsumer.AllLogs()[0])

	require.NoErrorf(t, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[0],
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp()),
		"Received logs did not match log records in file %s", expectedFile,
	)
}
