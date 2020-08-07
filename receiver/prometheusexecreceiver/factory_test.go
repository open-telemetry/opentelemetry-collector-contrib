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

package prometheusexecreceiver

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	sdconfig "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/receiver/prometheusreceiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

func TestCreateTraceAndMetricsReceiver(t *testing.T) {
	var (
		traceReceiver  component.TraceReceiver
		metricReceiver component.MetricsReceiver
	)

	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := &Factory{}
	receiverType := "prometheus_exec"
	factories.Receivers[configmodels.Type(receiverType)] = factory

	config, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	assert.NoError(t, err)
	assert.NotNil(t, config)

	receiver := config.Receivers[receiverType]

	// Test CreateTraceReceiver
	traceReceiver, err = factory.CreateTraceReceiver(context.Background(), zap.NewNop(), receiver, nil)

	assert.Equal(t, nil, traceReceiver)
	assert.Equal(t, configerror.ErrDataTypeIsNotSupported, err)

	// Test CreateMetricsReceiver
	metricReceiver, err = factory.CreateMetricsReceiver(context.Background(), zap.NewNop(), receiver, nil)
	assert.Equal(t, nil, err)

	wantPer := &prometheusExecReceiver{
		logger:   zap.NewNop(),
		config:   receiver.(*Config),
		consumer: nil,
		promReceiverConfig: &prometheusreceiver.Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "prometheus_exec",
				NameVal: "prometheus_exec",
			},
			PrometheusConfig: &promconfig.Config{
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						ScrapeInterval:  model.Duration(60 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						Scheme:          "http",
						MetricsPath:     "/metrics",
						JobName:         "prometheus_exec",
						HonorLabels:     false,
						HonorTimestamps: true,
						ServiceDiscoveryConfig: sdconfig.ServiceDiscoveryConfig{
							StaticConfigs: []*targetgroup.Group{
								{
									Targets: []model.LabelSet{
										{model.AddressLabel: model.LabelValue("localhost:0")},
									},
								},
							},
						},
					},
				},
			},
		},
		subprocessConfig: &subprocessmanager.SubprocessConfig{
			Command: "",
			Env:     []subprocessmanager.EnvConfig{},
		},
		originalPort:       0,
		prometheusReceiver: nil,
	}

	assert.Equal(t, wantPer, metricReceiver)
}
