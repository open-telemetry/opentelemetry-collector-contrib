// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/internal/metadata"
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

	sub, err := cm.Sub(component.NewID(metadata.Type).String())
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
	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "test").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	set := receivertest.NewNopCreateSettings()
	set.ID = component.NewID(metadata.Type)
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
						JobName:         metadata.Type,
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
