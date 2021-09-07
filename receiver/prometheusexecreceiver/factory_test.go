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
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

func TestCreateTraceAndMetricsReceiver(t *testing.T) {
	var (
		traceReceiver  component.TracesReceiver
		metricReceiver component.MetricsReceiver
	)

	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory

	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	receiver := cfg.Receivers[config.NewID(typeStr)]

	// Test CreateTracesReceiver
	traceReceiver, err = factory.CreateTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), receiver, nil)

	assert.Equal(t, nil, traceReceiver)
	assert.ErrorIs(t, err, componenterror.ErrDataTypeIsNotSupported)

	// Test CreateMetricsReceiver error because of lack of command
	_, err = factory.CreateMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), receiver, nil)
	assert.NotNil(t, err)

	// Test CreateMetricsReceiver
	receiver = cfg.Receivers[config.NewIDWithName(typeStr, "test")]
	metricReceiver, err = factory.CreateMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), receiver, nil)
	assert.Equal(t, nil, err)

	wantPer := &prometheusExecReceiver{
		params:   componenttest.NewNopReceiverCreateSettings(),
		config:   receiver.(*Config),
		consumer: nil,
		promReceiverConfig: &prometheusreceiver.Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "test")),
			PrometheusConfig: &promconfig.Config{
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						ScrapeInterval:  model.Duration(60 * time.Second),
						ScrapeTimeout:   model.Duration(10 * time.Second),
						Scheme:          "http",
						MetricsPath:     "/metrics",
						JobName:         "test",
						HonorLabels:     false,
						HonorTimestamps: true,
						ServiceDiscoveryConfigs: discovery.Configs{
							&discovery.StaticConfig{
								{
									Targets: []model.LabelSet{
										{model.AddressLabel: model.LabelValue("localhost:9104")},
									},
								},
							},
						},
					},
				},
			},
		},
		subprocessConfig: &subprocessmanager.SubprocessConfig{
			Command: "mysqld_exporter",
			Env:     []subprocessmanager.EnvConfig{},
		},
		port:               9104,
		prometheusReceiver: nil,
	}

	assert.Equal(t, wantPer, metricReceiver)
}
