// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

// Mock cadvisor
type mockCadvisor struct{}

func (c *mockCadvisor) GetMetrics() []pmetric.Metrics {
	md := pmetric.NewMetrics()
	return []pmetric.Metrics{md}
}

func (c *mockCadvisor) Shutdown() error {
	return nil
}

// Mock k8sapiserver
type mockK8sAPIServer struct{}

func (m *mockK8sAPIServer) Shutdown() error {
	return nil
}

func (m *mockK8sAPIServer) GetMetrics() []pmetric.Metrics {
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
	ctx := context.Background()

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
	_ = r.Start(context.Background(), nil)
	ctx := context.Background()
	r.k8sapiserver = &mockK8sAPIServer{}
	r.containerMetricsProvider = &mockCadvisor{}
	err = r.collectData(ctx)
	require.NoError(t, err)

	// test the case when cadvisor and k8sapiserver failed to initialize
	r.containerMetricsProvider = nil
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
	_ = r.Start(context.Background(), nil)
	r.containerMetricsProvider = &mockCadvisor{}
	r.k8sapiserver = &mockK8sAPIServer{}
	ctx := context.Background()

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
	_ = r.Start(context.Background(), nil)
	ctx := context.Background()

	r.containerMetricsProvider = &mockCadvisor{}
	err = r.collectData(ctx)
	require.NoError(t, err)

	// test the case when cadvisor and k8sapiserver failed to initialize
	r.containerMetricsProvider = nil
	err = r.collectData(ctx)
	require.Error(t, err)
}

func TestCollectDataWithSystemd(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ContainerOrchestrator = ci.EKS
	cfg.KubeConfigPath = "/tmp/kube-config"
	cfg.HostIP = "1.2.3.4"
	metricsReceiver, err := newAWSContainerInsightReceiver(
		componenttest.NewNopTelemetrySettings(),
		cfg,
		new(consumertest.MetricsSink),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsContainerInsightReceiver)
	_ = r.Start(context.Background(), nil)
	ctx := context.Background()

	r.containerMetricsProvider = &mockCadvisor{}
	err = r.collectData(ctx)
	require.NoError(t, err)
}

// MockHost is a mock implementation of component.Host
type MockHost struct {
	mock.Mock
}

func (m *MockHost) GetExtensions() map[component.ID]component.Component {
	args := m.Called()
	return args.Get(0).(map[component.ID]component.Component)
}

// MockConfigurer is a mock implementation of awsmiddleware.Configurer
type MockConfigurer struct {
	mock.Mock
}

func (m *MockConfigurer) Start(context.Context, component.Host) error {
	return nil
}

func (m *MockConfigurer) Shutdown(context.Context) error {
	return nil
}

func (m *MockHost) GetFactory(_ component.Kind, _ component.Type) component.Factory {
	return nil
}

func TestAWSContainerInsightReceiverStart(t *testing.T) {
	// Create a mock host
	mockHost := new(MockHost)
	testType, _ := component.NewType("awsmiddleware")

	// Create a mock configurer
	mockConfigurer := new(MockConfigurer)
	agenthealth, _ := component.NewType("agenthealth")
	// Set up the mock host to return a map with the mock configurer
	mockHost.On("GetExtensions").Return(map[component.ID]component.Component{
		component.NewID(testType): mockConfigurer,
	})

	statusCodeID := component.NewIDWithName(agenthealth, "statuscode")

	// Create a receiver instance
	config := &Config{
		CollectionInterval:    60,
		ContainerOrchestrator: "eks",
		MiddlewareID:          &statusCodeID,
	}
	consumer := consumertest.NewNop()
	receiver, err := newAWSContainerInsightReceiver(component.TelemetrySettings{}, config, consumer)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), mockHost)
	assert.Error(t, err)

	mockHost.AssertCalled(t, "GetExtensions")
}
