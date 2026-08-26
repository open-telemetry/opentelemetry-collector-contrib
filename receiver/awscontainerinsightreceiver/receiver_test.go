// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

// Mock cadvisor
type mockCadvisor struct{}

func (*mockCadvisor) GetMetrics() []pmetric.Metrics {
	md := pmetric.NewMetrics()
	return []pmetric.Metrics{md}
}

func (*mockCadvisor) Shutdown() error {
	return nil
}

// Mock k8sapiserver
type mockK8sAPIServer struct{}

func (*mockK8sAPIServer) Shutdown() error {
	return nil
}

func (*mockK8sAPIServer) GetMetrics() []pmetric.Metrics {
	md := pmetric.NewMetrics()
	return []pmetric.Metrics{md}
}

func TestReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAWSContainerInsightReceiver(
		componenttest.NewNopTelemetrySettings(),
		cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsContainerInsightReceiver)
	ctx := t.Context()

	err = r.Start(ctx, componenttest.NewNopHost())
	require.Error(t, err)

	err = r.Shutdown(ctx)
	require.NoError(t, err)
}

func TestCollectData(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAWSContainerInsightReceiver(
		componenttest.NewNopTelemetrySettings(),
		cfg,
		new(consumertest.MetricsSink),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsContainerInsightReceiver)
	_ = r.Start(t.Context(), componenttest.NewNopHost())
	ctx := t.Context()
	r.k8sapiserver = &mockK8sAPIServer{}
	r.cadvisor = &mockCadvisor{}
	err = r.collectData(ctx)
	require.NoError(t, err)

	// test the case when cadvisor and k8sapiserver failed to initialize
	r.cadvisor = nil
	r.k8sapiserver = nil
	err = r.collectData(ctx)
	require.Error(t, err)
}

func TestCollectDataWithErrConsumer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAWSContainerInsightReceiver(
		componenttest.NewNopTelemetrySettings(),
		cfg,
		consumertest.NewErr(errors.New("an error")),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsContainerInsightReceiver)
	_ = r.Start(t.Context(), componenttest.NewNopHost())
	r.cadvisor = &mockCadvisor{}
	r.k8sapiserver = &mockK8sAPIServer{}
	ctx := t.Context()

	err = r.collectData(ctx)
	require.Error(t, err)
}

func TestCollectDataWithECS(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ContainerOrchestrator = ci.ECS
	metricsReceiver, err := newAWSContainerInsightReceiver(
		componenttest.NewNopTelemetrySettings(),
		cfg,
		new(consumertest.MetricsSink),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsContainerInsightReceiver)
	_ = r.Start(t.Context(), componenttest.NewNopHost())
	ctx := t.Context()

	r.cadvisor = &mockCadvisor{}
	err = r.collectData(ctx)
	require.NoError(t, err)

	// test the case when cadvisor and k8sapiserver failed to initialize
	r.cadvisor = nil
	err = r.collectData(ctx)
	require.Error(t, err)
}
