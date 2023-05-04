// Copyright The OpenTelemetry Authors
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
	"path/filepath"
	"testing"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

func TestCreateTraceAndMetricsReceiver(t *testing.T) {
	var (
		traceReceiver  receiver.Traces
		metricReceiver receiver.Metrics
	)

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewID(typeStr).String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Test CreateTracesReceiver
	traceReceiver, err = factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)

	assert.Equal(t, nil, traceReceiver)
	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)

	// Test error because of lack of command
	assert.Error(t, component.ValidateConfig(cfg))

	// Test CreateMetricsReceiver
	sub, err = cm.Sub(component.NewIDWithName(typeStr, "test").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	set := receivertest.NewNopCreateSettings()
	set.ID = component.NewID(typeStr)
	metricReceiver, err = factory.CreateMetricsReceiver(context.Background(), set, cfg, nil)
	assert.Equal(t, nil, err)

	wantPer := &prometheusExecReceiver{
		params:   set,
		config:   cfg.(*Config),
		consumer: nil,
		promReceiverConfig: &prometheusreceiver.Config{
			PrometheusConfig: &promconfig.Config{
				ScrapeConfigs: []*promconfig.ScrapeConfig{
					{
						ScrapeInterval:  model.Duration(defaultCollectionInterval),
						ScrapeTimeout:   model.Duration(defaultTimeoutInterval),
						Scheme:          "http",
						MetricsPath:     "/metrics",
						JobName:         typeStr,
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
