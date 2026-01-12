// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"google.golang.org/grpc/encoding/gzip"

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
				QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				Protocol:      "grpc",
				PrivateKey:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				AppName:       "APP_NAME",
				// Deprecated: [v0.47.0] SubSystem will remove in the next version
				SubSystem:       "SUBSYSTEM_NAME",
				TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
				DomainSettings: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Compression: configcompression.TypeGzip,
					},
					AcceptEncoding: gzip.Name,
				},
				Metrics: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint:        "https://",
						Compression:     configcompression.TypeGzip,
						WriteBufferSize: 512 * 1024,
					},
					AcceptEncoding: gzip.Name,
				},
				Logs: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint:    "https://",
						Compression: configcompression.TypeGzip,
					},
					AcceptEncoding: gzip.Name,
				},
				Traces: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx:4317",
						Compression: configcompression.TypeGzip,
						TLS: configtls.ClientConfig{
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
					AcceptEncoding: gzip.Name,
				},
				RateLimiter: RateLimiterConfig{
					Enabled:   true,
					Threshold: 10,
					Duration:  time.Minute,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "all"),
			expected: &Config{
				QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				Protocol:      "grpc",
				PrivateKey:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				AppName:       "APP_NAME",
				// Deprecated: [v0.47.0] SubSystem will remove in the next version
				SubSystem:       "SUBSYSTEM_NAME",
				TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
				DomainSettings: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Compression: configcompression.TypeGzip,
					},
					AcceptEncoding: gzip.Name,
				},
				Metrics: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint:        "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx:4317",
						Compression:     configcompression.TypeGzip,
						WriteBufferSize: 512 * 1024,
					},
					AcceptEncoding: gzip.Name,
				},
				Logs: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx:4317",
						Compression: configcompression.TypeGzip,
					},
					AcceptEncoding: gzip.Name,
				},
				Traces: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint:    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx:4317",
						Compression: configcompression.TypeGzip,
						TLS: configtls.ClientConfig{
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
					AcceptEncoding: gzip.Name,
				},
				AppNameAttributes:   []string{"service.namespace", "k8s.namespace.name"},
				SubSystemAttributes: []string{"service.name", "k8s.deployment.name", "k8s.statefulset.name", "k8s.daemonset.name", "k8s.cronjob.name", "k8s.job.name", "k8s.container.name"},
				RateLimiter: RateLimiterConfig{
					Enabled:   true,
					Threshold: 10,
					Duration:  time.Minute,
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
	assert.NoError(t, te.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, te.shutdown(t.Context()))
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
	require.NoError(t, me.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, me.shutdown(t.Context()))
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
	require.NoError(t, le.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, le.shutdown(t.Context()))
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
	assert.NoError(t, te.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, te.shutdown(t.Context()))

	me, err := newMetricsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create metrics exporter")
	require.NoError(t, me.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, me.shutdown(t.Context()))

	le, err := newLogsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, le, "failed to create logs exporter")
	require.NoError(t, le.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, le.shutdown(t.Context()))
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
	assert.NoError(t, te.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, te.shutdown(t.Context()))

	me, err := newMetricsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, me, "failed to create metrics exporter")
	require.NoError(t, me.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, me.shutdown(t.Context()))

	le, err := newLogsExporter(cfg, params)
	require.NoError(t, err)
	require.NotNil(t, le, "failed to create logs exporter")
	require.NoError(t, le.start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, le.shutdown(t.Context()))
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

func TestGetDomainGrpcSettings(t *testing.T) {
	tests := []struct {
		name             string
		domain           string
		privateLink      bool
		expectedEndpoint string
	}{
		{
			name:             "Standard domain without PrivateLink",
			domain:           "coralogix.com",
			privateLink:      false,
			expectedEndpoint: "ingress.coralogix.com:443",
		},
		{
			name:             "Standard domain with PrivateLink",
			domain:           "coralogix.com",
			privateLink:      true,
			expectedEndpoint: "ingress.private.coralogix.com:443",
		},
		{
			name:             "EU2 domain without PrivateLink",
			domain:           "eu2.coralogix.com",
			privateLink:      false,
			expectedEndpoint: "ingress.eu2.coralogix.com:443",
		},
		{
			name:             "EU2 domain with PrivateLink",
			domain:           "eu2.coralogix.com",
			privateLink:      true,
			expectedEndpoint: "ingress.private.eu2.coralogix.com:443",
		},
		{
			name:             "AP1 domain without PrivateLink",
			domain:           "coralogix.in",
			privateLink:      false,
			expectedEndpoint: "ingress.coralogix.in:443",
		},
		{
			name:             "AP1 domain with PrivateLink",
			domain:           "coralogix.in",
			privateLink:      true,
			expectedEndpoint: "ingress.private.coralogix.in:443",
		},
		{
			name:             "US1 domain with PrivateLink",
			domain:           "coralogix.us",
			privateLink:      true,
			expectedEndpoint: "ingress.private.coralogix.us:443",
		},
		{
			name:             "Domain already contains private prefix with PrivateLink",
			domain:           "private.coralogix.com",
			privateLink:      true,
			expectedEndpoint: "ingress.private.coralogix.com:443",
		},
		{
			name:             "Domain already contains private prefix without PrivateLink",
			domain:           "private.coralogix.com",
			privateLink:      false,
			expectedEndpoint: "ingress.private.coralogix.com:443",
		},
		{
			name:             "EU2 domain already contains private with PrivateLink",
			domain:           "private.eu2.coralogix.com",
			privateLink:      true,
			expectedEndpoint: "ingress.private.eu2.coralogix.com:443",
		},
		{
			name:             "Domain contains private in middle with PrivateLink",
			domain:           "eu2.private.coralogix.com",
			privateLink:      true,
			expectedEndpoint: "ingress.eu2.private.coralogix.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Domain:      tt.domain,
				PrivateLink: tt.privateLink,
				DomainSettings: TransportConfig{
					ClientConfig: configgrpc.ClientConfig{
						Compression: configcompression.TypeGzip,
					},
				},
			}

			endpoint := setDomainGrpcSettings(cfg)
			assert.Equal(t, tt.expectedEndpoint, endpoint)
		})
	}
}

func TestCreateExportersWithBatcher(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Domain = "localhost"
	cfg.PrivateKey = "test-key"
	cfg.AppName = "test-app"
	cfg.QueueSettings.GetOrInsertDefault()
	cfg.QueueSettings.Get().Batch = configoptional.Some(exporterhelper.BatchConfig{
		FlushTimeout: 1 * time.Second,
		MinSize:      100,
	})

	// Test traces exporter
	t.Run("traces_with_batcher", func(t *testing.T) {
		set := exportertest.NewNopSettings(metadata.Type)
		exp, err := factory.CreateTraces(t.Context(), set, cfg)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	// Test metrics exporter
	t.Run("metrics_with_batcher", func(t *testing.T) {
		set := exportertest.NewNopSettings(metadata.Type)
		exp, err := factory.CreateMetrics(t.Context(), set, cfg)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	// Test logs exporter
	t.Run("logs_with_batcher", func(t *testing.T) {
		set := exportertest.NewNopSettings(metadata.Type)
		exp, err := factory.CreateLogs(t.Context(), set, cfg)
		require.NoError(t, err)
		require.NotNil(t, exp)
	})
}

func TestGetAcceptEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		acceptEncoding   string
		expectedEncoding string
	}{
		{
			name:             "empty_returns_empty",
			acceptEncoding:   "",
			expectedEncoding: "",
		},
		{
			name:             "explicit_gzip",
			acceptEncoding:   gzip.Name,
			expectedEncoding: gzip.Name,
		},
		{
			name:             "custom_encoding",
			acceptEncoding:   "snappy",
			expectedEncoding: "snappy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TransportConfig{
				AcceptEncoding: tt.acceptEncoding,
			}
			assert.Equal(t, tt.expectedEncoding, cfg.GetAcceptEncoding())
		})
	}
}

func TestConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "valid_grpc_config",
			config: &Config{
				Protocol:   "grpc",
				Domain:     "coralogix.com",
				PrivateKey: "test-key",
				AppName:    "test-app",
				Profiles: configgrpc.ClientConfig{
					Endpoint: "ingress.coralogix.com:443",
				},
			},
			expectedErr: "",
		},
		{
			name: "no_protocol_defaults_to_grpc",
			config: &Config{
				Domain:     "coralogix.com",
				PrivateKey: "test-key",
				AppName:    "test-app",
				Profiles: configgrpc.ClientConfig{
					Endpoint: "ingress.coralogix.com:443",
				},
			},
			expectedErr: "",
		},
		{
			name: "invalid_protocol",
			config: &Config{
				Protocol:   "tcp",
				Domain:     "coralogix.com",
				PrivateKey: "test-key",
				AppName:    "test-app",
			},
			expectedErr: "protocol must be grpc or http",
		},
		{
			name: "invalid_http_with_profiles",
			config: &Config{
				Protocol:   "http",
				Domain:     "coralogix.com",
				PrivateKey: "test-key",
				AppName:    "test-app",
				Profiles: configgrpc.ClientConfig{
					Endpoint: "ingress.coralogix.com:443",
				},
			},
			expectedErr: "profiles signal is not supported with HTTP protocol",
		},
		{
			name: "valid_http_without_profiles",
			config: &Config{
				Protocol:   "http",
				Domain:     "coralogix.com",
				PrivateKey: "test-key",
				AppName:    "test-app",
			},
			expectedErr: "",
		},
		{
			name: "valid_gzip_accept_encoding",
			config: &Config{
				Protocol:   "grpc",
				Domain:     "coralogix.com",
				PrivateKey: "test-key",
				AppName:    "test-app",
				Traces: TransportConfig{
					AcceptEncoding: gzip.Name,
				},
			},
			expectedErr: "",
		},
		{
			name: "invalid_accept_encoding",
			config: &Config{
				Protocol:   "grpc",
				Domain:     "coralogix.com",
				PrivateKey: "test-key",
				AppName:    "test-app",
				Traces: TransportConfig{
					AcceptEncoding: "invalid-encoding",
				},
			},
			expectedErr: "traces.accept_encoding: unsupported compression encoding",
		},
		{
			name: "empty_accept_encoding_is_valid",
			config: &Config{
				Protocol:   "grpc",
				Domain:     "coralogix.com",
				PrivateKey: "test-key",
				AppName:    "test-app",
				Traces: TransportConfig{
					AcceptEncoding: "",
				},
			},
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}
