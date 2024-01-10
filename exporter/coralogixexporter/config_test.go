// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"path/filepath"
	"testing"

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
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				PrivateKey:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				AppName:       "APP_NAME",
				// Deprecated: [v0.47.0] SubSystem will remove in the next version
				SubSystem:       "SUBSYSTEM_NAME",
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
				DomainSettings: configgrpc.GRPCClientSettings{
					Compression: configcompression.Gzip,
				},
				Metrics: configgrpc.GRPCClientSettings{
					Endpoint:        "https://",
					Compression:     configcompression.Gzip,
					WriteBufferSize: 512 * 1024,
				},
				Logs: configgrpc.GRPCClientSettings{
					Endpoint:    "https://",
					Compression: configcompression.Gzip,
				},
				Traces: configgrpc.GRPCClientSettings{
					Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					Compression: configcompression.Gzip,
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting:         configtls.TLSSetting{},
						Insecure:           false,
						InsecureSkipVerify: false,
						ServerName:         "",
					},
					ReadBufferSize:  0,
					WriteBufferSize: 0,
					WaitForReady:    false,
					BalancerName:    "",
				},
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Endpoint: "https://",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting:         configtls.TLSSetting{},
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
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				PrivateKey:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				AppName:       "APP_NAME",
				// Deprecated: [v0.47.0] SubSystem will remove in the next version
				SubSystem:       "SUBSYSTEM_NAME",
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
				DomainSettings: configgrpc.GRPCClientSettings{
					Compression: configcompression.Gzip,
				},
				Metrics: configgrpc.GRPCClientSettings{
					Endpoint:        "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					Compression:     configcompression.Gzip,
					WriteBufferSize: 512 * 1024,
				},
				Logs: configgrpc.GRPCClientSettings{
					Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					Compression: configcompression.Gzip,
				},
				Traces: configgrpc.GRPCClientSettings{
					Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					Compression: configcompression.Gzip,
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting:         configtls.TLSSetting{},
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
				GRPCClientSettings: configgrpc.GRPCClientSettings{
					Endpoint: "https://",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting:         configtls.TLSSetting{},
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
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
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	te, err := newTracesExporter(cfg, params)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
	assert.NoError(t, te.start(context.Background(), componenttest.NewNopHost()))
}

func TestMetricsExporter(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "metrics.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "metrics").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	require.NoError(t, component.ValidateConfig(cfg))

	params := exportertest.NewNopCreateSettings()

	me, err := newMetricsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create metrics exporter")
	require.NoError(t, me.start(context.Background(), componenttest.NewNopHost()))
}

func TestLogsExporter(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "logs.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "logs").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	require.NoError(t, component.ValidateConfig(cfg))

	params := exportertest.NewNopCreateSettings()

	me, err := newLogsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create logs exporter")
	require.NoError(t, me.start(context.Background(), componenttest.NewNopHost()))
}

func TestDomainWithAllExporters(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "domain").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	te, err := newTracesExporter(cfg, params)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
	assert.NoError(t, te.start(context.Background(), componenttest.NewNopHost()))

	me, err := newMetricsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create metrics exporter")
	require.NoError(t, me.start(context.Background(), componenttest.NewNopHost()))

	le, err := newLogsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, le, "failed to create logs exporter")
	require.NoError(t, le.start(context.Background(), componenttest.NewNopHost()))
}

func TestEndpoindsAndDomainWithAllExporters(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "domain_endoints").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	params := exportertest.NewNopCreateSettings()
	te, err := newTracesExporter(cfg, params)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
	assert.NoError(t, te.start(context.Background(), componenttest.NewNopHost()))

	me, err := newMetricsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create metrics exporter")
	require.NoError(t, me.start(context.Background(), componenttest.NewNopHost()))

	le, err := newLogsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, le, "failed to create logs exporter")
	require.NoError(t, le.start(context.Background(), componenttest.NewNopHost()))
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
}
