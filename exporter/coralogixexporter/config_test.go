// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				QueueSettings: exporterhelper.NewDefaultQueueConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				PrivateKey:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				AppName:       "APP_NAME",
				// Deprecated: [v0.47.0] SubSystem will remove in the next version
				SubSystem:       "SUBSYSTEM_NAME",
				TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
				DomainSettings: configgrpc.ClientConfig{
					Compression: configcompression.TypeGzip,
				},
				Metrics: configgrpc.ClientConfig{
					Endpoint:        "https://",
					Compression:     configcompression.TypeGzip,
					WriteBufferSize: 512 * 1024,
				},
				Logs: configgrpc.ClientConfig{
					Endpoint:    "https://",
					Compression: configcompression.TypeGzip,
				},
				Traces: configgrpc.ClientConfig{
					Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					Compression: configcompression.TypeGzip,
					TLSSetting: configtls.ClientConfig{
						Config:             configtls.Config{},
						Insecure:           false,
						InsecureSkipVerify: false,
						ServerName:         "",
					},
					ReadBufferSize:  0,
					WriteBufferSize: 0,
					WaitForReady:    false,
					BalancerName:    "",
				},
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "https://",
					TLSSetting: configtls.ClientConfig{
						Config:             configtls.Config{},
						Insecure:           false,
						InsecureSkipVerify: false,
						ServerName:         "",
					},
					ReadBufferSize:  0,
					WriteBufferSize: 0,
					WaitForReady:    false,
					Headers: map[string]configopaque.String{
						"ACCESS_TOKEN": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
						"appName":      "APP_NAME",
					},
					BalancerName: "",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "all"),
			expected: &Config{
				QueueSettings: exporterhelper.NewDefaultQueueConfig(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				PrivateKey:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				AppName:       "APP_NAME",
				// Deprecated: [v0.47.0] SubSystem will remove in the next version
				SubSystem:       "SUBSYSTEM_NAME",
				TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
				DomainSettings: configgrpc.ClientConfig{
					Compression: configcompression.TypeGzip,
				},
				Metrics: configgrpc.ClientConfig{
					Endpoint:        "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					Compression:     configcompression.TypeGzip,
					WriteBufferSize: 512 * 1024,
				},
				Logs: configgrpc.ClientConfig{
					Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					Compression: configcompression.TypeGzip,
				},
				Traces: configgrpc.ClientConfig{
					Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					Compression: configcompression.TypeGzip,
					TLSSetting: configtls.ClientConfig{
						Config:             configtls.Config{},
						Insecure:           false,
						InsecureSkipVerify: false,
						ServerName:         "",
					},
					ReadBufferSize:  0,
					WriteBufferSize: 0,
					WaitForReady:    false,
					BalancerName:    "",
				},
				AppNameAttributes:   []string{"service.namespace", "k8s.namespace.name"},
				SubSystemAttributes: []string{"service.name", "k8s.deployment.name", "k8s.statefulset.name", "k8s.daemonset.name", "k8s.cronjob.name", "k8s.job.name", "k8s.container.name"},
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "https://",
					TLSSetting: configtls.ClientConfig{
						Config:             configtls.Config{},
						Insecure:           false,
						InsecureSkipVerify: false,
						ServerName:         "",
					},
					ReadBufferSize:  0,
					WriteBufferSize: 0,
					WaitForReady:    false,
					Headers: map[string]configopaque.String{
						"ACCESS_TOKEN": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
						"appName":      "APP_NAME",
					},
					BalancerName: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestTraceExporter(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	params := exportertest.NewNopSettings(metadata.Type)
	te, err := newTracesExporter(cfg, params)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
	assert.NoError(t, te.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, te.shutdown(context.Background()))
}

func TestMetricsExporter(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "metrics.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "metrics").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	params := exportertest.NewNopSettings(metadata.Type)

	me, err := newMetricsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create metrics exporter")
	require.NoError(t, me.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, me.shutdown(context.Background()))
}

func TestLogsExporter(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "logs.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "logs").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	params := exportertest.NewNopSettings(metadata.Type)

	le, err := newLogsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, le, "failed to create logs exporter")
	require.NoError(t, le.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, le.shutdown(context.Background()))
}

func TestDomainWithAllExporters(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "domain").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	params := exportertest.NewNopSettings(metadata.Type)
	te, err := newTracesExporter(cfg, params)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
	assert.NoError(t, te.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, te.shutdown(context.Background()))

	me, err := newMetricsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create metrics exporter")
	require.NoError(t, me.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, me.shutdown(context.Background()))

	le, err := newLogsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, le, "failed to create logs exporter")
	require.NoError(t, le.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, le.shutdown(context.Background()))
}

func TestEndpointsAndDomainWithAllExporters(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "domain_endpoints").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	params := exportertest.NewNopSettings(metadata.Type)
	te, err := newTracesExporter(cfg, params)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
	assert.NoError(t, te.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, te.shutdown(context.Background()))

	me, err := newMetricsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create metrics exporter")
	require.NoError(t, me.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, me.shutdown(context.Background()))

	le, err := newLogsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, le, "failed to create logs exporter")
	require.NoError(t, le.start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, le.shutdown(context.Background()))
}

func TestGetMetadataFromResource(t *testing.T) {
	r1 := pcommon.NewResource()
	r1.Attributes().PutStr("k8s.node.name", "node-test")
	r1.Attributes().PutStr("k8s.container.name", "container-test")
	r1.Attributes().PutStr("k8s.deployment.name", "deployment-test")
	r1.Attributes().PutStr("k8s.namespace.name", "namespace-test")

	r2 := pcommon.NewResource()
	r2.Attributes().PutStr("k8s.node.name", "node-test")
	r2.Attributes().PutStr("k8s.namespace.name", "namespace-test")

	r3 := pcommon.NewResource()
	r3.Attributes().PutStr("cx.application.name", "application")
	r3.Attributes().PutStr("cx.subsystem.name", "subsystem")

	c := &Config{
		AppNameAttributes:   []string{"k8s.container.name", "k8s.deployment.name", "k8s.node.name"},
		SubSystemAttributes: []string{"k8s.namespace.name", "k8s.node.name"},
	}

	appName, subSystemName := c.getMetadataFromResource(r1)
	assert.Equal(t, "container-test", appName)
	assert.Equal(t, "namespace-test", subSystemName)

	appName, subSystemName = c.getMetadataFromResource(r2)
	assert.Equal(t, "node-test", appName)
	assert.Equal(t, "namespace-test", subSystemName)

	appName, subSystemName = c.getMetadataFromResource(r3)
	assert.Equal(t, "application", appName)
	assert.Equal(t, "subsystem", subSystemName)
}

func TestCreateExportersWithBatcher(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Domain = "localhost"
	cfg.PrivateKey = "test-key"
	cfg.AppName = "test-app"
	cfg.QueueSettings.Enabled = true
	cfg.QueueSettings.Batch = &exporterhelper.BatchConfig{
		FlushTimeout: 1 * time.Second,
		MinSize:      100,
	}

	// Test traces exporter
	t.Run("traces_with_batcher", func(t *testing.T) {
		set := exportertest.NewNopSettings(metadata.Type)
		exp, err := factory.CreateTraces(context.Background(), set, cfg)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	// Test metrics exporter
	t.Run("metrics_with_batcher", func(t *testing.T) {
		set := exportertest.NewNopSettings(metadata.Type)
		exp, err := factory.CreateMetrics(context.Background(), set, cfg)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	// Test logs exporter
	t.Run("logs_with_batcher", func(t *testing.T) {
		set := exportertest.NewNopSettings(metadata.Type)
		exp, err := factory.CreateLogs(context.Background(), set, cfg)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})
}
