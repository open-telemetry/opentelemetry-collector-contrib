// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tanzuobservabilityexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[exporterType] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	actual, ok := cfg.Exporters[component.NewID("tanzuobservability")]
	require.True(t, ok)
	expected := &Config{
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:40001"},
		},
		Metrics: MetricsConfig{
			HTTPClientSettings:    confighttp.HTTPClientSettings{Endpoint: "http://localhost:2916"},
			ResourceAttrsIncluded: true,
			AppTagsExcluded:       true,
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 2,
			QueueSize:    10,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:             true,
			InitialInterval:     10 * time.Second,
			MaxInterval:         60 * time.Second,
			MaxElapsedTime:      10 * time.Minute,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
		},
	}
	assert.Equal(t, expected, actual)
}

func TestConfigRequiresValidEndpointUrl(t *testing.T) {
	c := &Config{
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http#$%^&#$%&#"},
		},
	}
	assert.Error(t, c.Validate())
}

func TestMetricsConfigRequiresValidEndpointUrl(t *testing.T) {
	c := &Config{
		Metrics: MetricsConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http#$%^&#$%&#"},
		},
	}

	assert.Error(t, c.Validate())
}

func TestDifferentHostNames(t *testing.T) {
	c := &Config{
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:30001"},
		},
		Metrics: MetricsConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://foo.com:2878"},
		},
	}
	assert.Error(t, c.Validate())
}

func TestConfigNormal(t *testing.T) {
	c := &Config{
		Traces: TracesConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:40001"},
		},
		Metrics: MetricsConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost:2916"},
		},
	}
	assert.NoError(t, c.Validate())
}

func TestMetricConfig(t *testing.T) {
	c := &Config{
		Metrics: MetricsConfig{},
	}
	assert.NoError(t, c.Validate())
	assert.False(t, c.Metrics.ResourceAttrsIncluded)
	assert.False(t, c.Metrics.AppTagsExcluded)

	c = &Config{
		Metrics: MetricsConfig{
			ResourceAttrsIncluded: true,
			AppTagsExcluded:       true,
		},
	}
	assert.NoError(t, c.Validate())
	assert.True(t, c.Metrics.ResourceAttrsIncluded)
	assert.True(t, c.Metrics.AppTagsExcluded)
}
