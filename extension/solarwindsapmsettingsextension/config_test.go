// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "apm.collector.na-01.cloud.solarwinds.com:443",
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "apm.collector.na-02.cloud.solarwinds.com:443",
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "apm.collector.eu-01.cloud.solarwinds.com:443",
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "4"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "apm.collector.apj-01.cloud.solarwinds.com:443",
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "5"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "apm.collector.na-01.st-ssp.solarwinds.com:443",
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "6"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "apm.collector.na-01.dev-ssp.solarwinds.com:443",
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "7"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "8"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "9"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "10"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "11"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "12"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "13"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "14"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "15"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "16"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "17"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      ":",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "18"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "::",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "19"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      ":name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "20"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "token:",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "21"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "token:name",
				Interval: MinimumInterval,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "22"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "token:name",
				Interval: MaximumInterval,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "23"),
			expected: &Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: DefaultEndpoint,
				},
				Key:      "token:name",
				Interval: MinimumInterval,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
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

func TestResolveServiceNameBestEffortNoEnv(t *testing.T) {
	// Without any environment variables
	require.Empty(t, resolveServiceNameBestEffort())
}

// With OTEL_SERVICE_NAME only
func TestResolveServiceNameBestEffortOnlyOtelService(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "otel_ser1")
	require.Equal(t, "otel_ser1", resolveServiceNameBestEffort())
}

// With AWS_LAMBDA_FUNCTION_NAME only
func TestResolveServiceNameBestEffortOnlyAwsLambda(t *testing.T) {
	t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "lambda")
	require.Equal(t, "lambda", resolveServiceNameBestEffort())
}

// With both
func TestResolveServiceNameBestEffortBoth(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "otel_ser1")
	t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "lambda")
	require.Equal(t, "otel_ser1", resolveServiceNameBestEffort())
}
