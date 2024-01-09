// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, component.Type("sumologic"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	qs := exporterhelper.NewDefaultQueueSettings()
	qs.Enabled = false

	assert.Equal(t, cfg, &Config{
		CompressEncoding:   "gzip",
		MaxRequestBodySize: 1_048_576,
		LogFormat:          "json",
		MetricFormat:       "prometheus",
		SourceCategory:     "",
		SourceName:         "",
		SourceHost:         "",
		Client:             "otelcol",
		GraphiteTemplate:   "%{_metric_}",

		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: 5 * time.Second,
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: qs,
	})
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}
