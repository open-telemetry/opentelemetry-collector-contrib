// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package receivercreator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
	zapObserver "go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

type mockMetricsConsumer struct {
	Metrics      []consumerdata.MetricsData
	TotalMetrics int
}

var _ consumer.MetricsConsumerOld = &mockMetricsConsumer{}

func (p *mockMetricsConsumer) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	p.Metrics = append(p.Metrics, md)
	p.TotalMetrics += len(md.Metrics)
	return nil
}

type mockObserver struct {
}

func (m *mockObserver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (m *mockObserver) Shutdown(ctx context.Context) error {
	return nil
}

var _ component.ServiceExtension = (*mockObserver)(nil)

func (m *mockObserver) ListAndWatch(notify observer.Notify) {
	notify.OnAdd([]observer.Endpoint{portEndpoint})
}

var _ observer.Observable = (*mockObserver)(nil)

func TestMockedEndToEnd(t *testing.T) {
	host, cfg := exampleCreatorFactory(t)
	host.extensions = map[configmodels.Extension]component.ServiceExtension{
		&configmodels.ExtensionSettings{
			TypeVal: "mock_observer",
			NameVal: "mock_observer",
		}: &mockObserver{},
	}
	dynCfg := cfg.Receivers["receiver_creator/1"]
	factory := &Factory{}
	mockConsumer := &mockMetricsConsumer{}
	rcvr, err := factory.CreateMetricsReceiver(context.Background(), zap.NewNop(), dynCfg, mockConsumer)
	require.NoError(t, err)
	dyn := rcvr.(*receiverCreator)
	require.NoError(t, rcvr.Start(context.Background(), host))

	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			assert.NoError(t, rcvr.Shutdown(context.Background()))
		})
	}

	defer shutdown()

	require.Eventuallyf(t, func() bool {
		return dyn.observerHandler.receiversByEndpointID.Size() == 1
	}, 1*time.Second, 100*time.Millisecond, "expected 1 receiver but got %v", dyn.observerHandler.receiversByEndpointID)

	// Test that we can send metrics.
	for _, receiver := range dyn.observerHandler.receiversByEndpointID.Values() {
		example := receiver.(*config.ExampleReceiverProducer)
		assert.NoError(t, example.MetricsConsumer.ConsumeMetricsData(context.Background(), consumerdata.MetricsData{
			Node: &commonpb.Node{
				ServiceInfo: &commonpb.ServiceInfo{Name: "dynamictest"},
				LibraryInfo: &commonpb.LibraryInfo{},
				Identifier:  &commonpb.ProcessIdentifier{},
				Attributes: map[string]string{
					"attr": "1",
				},
			},
			Resource: &resourcepb.Resource{Type: "test"},
			Metrics: []*metricspb.Metric{
				{
					MetricDescriptor: &metricspb.MetricDescriptor{
						Name:        "my-metric",
						Description: "My metric",
						Type:        metricspb.MetricDescriptor_GAUGE_INT64,
					},
					Timeseries: []*metricspb.TimeSeries{
						{
							Points: []*metricspb.Point{
								{Value: &metricspb.Point_Int64Value{Int64Value: 123}},
							},
						},
					},
				},
			}}))
	}

	// TODO: Will have to rework once receivers are started asynchronously to Start().
	assert.Len(t, mockConsumer.Metrics, 1)

	shutdown()

	assert.True(t, dyn.observerHandler.receiversByEndpointID.Values()[0].(*config.ExampleReceiverProducer).Stopped)
}

func TestLoggingHost(t *testing.T) {
	core, obs := zapObserver.New(zap.ErrorLevel)
	host := &loggingHost{
		Host:   componenttest.NewNopHost(),
		logger: zap.New(core),
	}
	host.ReportFatalError(errors.New("runtime error"))
	require.Equal(t, 1, obs.Len())
	log := obs.All()[0]
	assert.Equal(t, "receiver reported a fatal error", log.Message)
	assert.Equal(t, "runtime error", log.ContextMap()["error"])
}
