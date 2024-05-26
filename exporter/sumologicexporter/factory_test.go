// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, metadata.Type)
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	qs := exporterhelper.NewDefaultQueueSettings()
	qs.Enabled = false

	assert.Equal(t, cfg, &Config{
		MaxRequestBodySize: 1_048_576,
		LogFormat:          "otlp",
		MetricFormat:       "otlp",
		Client:             "otelcol",

		ClientConfig: confighttp.ClientConfig{
			Timeout:     30 * time.Second,
			Compression: "gzip",
			Auth: &configauth.Authentication{
				AuthenticatorID: component.NewID(metadata.Type),
			},
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: qs,
	})

	assert.NoError(t, component.ValidateConfig(cfg))
}
