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
	"sync"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/component"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configcheck"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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

func TestEndToEnd(t *testing.T) {
	host, cfg := exampleCreatorFactory(t)
	dynCfg := cfg.Receivers["receiver_creator/1"]
	factory := &Factory{}
	mockConsumer := &mockMetricsConsumer{}
	dynReceiver, err := factory.CreateMetricsReceiver(zap.NewNop(), dynCfg, mockConsumer)
	require.NoError(t, err)
	require.NoError(t, dynReceiver.Start(host))

	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			assert.NoError(t, dynReceiver.Shutdown())
		})
	}

	defer shutdown()

	dyn := dynReceiver.(*receiverCreator)
	assert.Len(t, dyn.receivers, 1)

	// Test that we can send metrics.
	for _, receiver := range dyn.receivers {
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
	assert.Equal(t, "my-metric", mockConsumer.Metrics[0].Metrics[0].MetricDescriptor.Name)

	shutdown()
}

func Test_loadAndCreateRuntimeReceiver(t *testing.T) {
	host, cfg := exampleCreatorFactory(t)
	dynCfg := cfg.Receivers["receiver_creator/1"]
	factory := &Factory{}
	dynReceiver, err := factory.CreateMetricsReceiver(zap.NewNop(), dynCfg, &mockMetricsConsumer{})
	require.NoError(t, err)
	dr := dynReceiver.(*receiverCreator)
	exampleFactory := host.GetFactory(component.KindReceiver, "examplereceiver").(component.ReceiverFactoryOld)
	assert.NotNil(t, exampleFactory)
	subConfig := dr.cfg.subreceiverConfigs["examplereceiver/1"]
	require.NotNil(t, subConfig)
	loadedConfig, err := dr.loadRuntimeReceiverConfig(exampleFactory, subConfig, userConfigMap{
		"endpoint": "localhost:12345",
	})
	require.NoError(t, err)
	assert.NotNil(t, loadedConfig)
	exampleConfig := loadedConfig.(*config.ExampleReceiver)
	// Verify that the overridden endpoint is used instead of the one in the config file.
	assert.Equal(t, "localhost:12345", exampleConfig.Endpoint)
	assert.Equal(t, "receiver_creator/1/examplereceiver/1{endpoint=\"localhost:12345\"}", exampleConfig.Name())

	// Test that metric receiver can be created from loaded config.
	t.Run("test create receiver from loaded config", func(t *testing.T) {
		recvr, err := dr.createRuntimeReceiver(exampleFactory, loadedConfig)
		require.NoError(t, err)
		assert.NotNil(t, recvr)
		exampleReceiver := recvr.(*config.ExampleReceiverProducer)
		assert.Equal(t, dr, exampleReceiver.MetricsConsumer)
	})
}
