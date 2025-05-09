// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8sobserver

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	k8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const (
	testKubeConfig                = "/tmp/kube-config-otelcol-e2e-testing"
	namespacedCollectorObjectsDir = "./testdata/e2e/namespaced/collector/"
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
	require.NoError(t, err, "failed creating metrics receiver")
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

	// verify that we eventually receive metrics by the receiver created via the receiver creator receiving the endpoint information from the k8sobserver
	require.Eventuallyf(t, func() bool {
		for _, metric := range metricsConsumer.AllMetrics() {
			for i := 0; i < metric.ResourceMetrics().Len(); i++ {
				res := metric.ResourceMetrics().At(i)
				for j := 0; j < res.ScopeMetrics().Len(); j++ {
					if res.ScopeMetrics().At(j).Scope().Name() == "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver" {
						return true
					}
				}
			}
		}
		return false
	}, 1*time.Minute, 1*time.Second,
		"Timeout: failed to receive metrics in 1 minute")
}
