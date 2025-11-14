// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mezmoexporter

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, metadata.Type)
}

var (
	defaultMaxIdleConns        = http.DefaultTransport.(*http.Transport).MaxIdleConns
	defaultMaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost
	defaultMaxConnsPerHost     = http.DefaultTransport.(*http.Transport).MaxConnsPerHost
	defaultIdleConnTimeout     = http.DefaultTransport.(*http.Transport).IdleConnTimeout
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		IngestURL: defaultIngestURL,
		IngestKey: "",

		ClientConfig: confighttp.ClientConfig{
			Timeout:             5 * time.Second,
			MaxIdleConns:        defaultMaxIdleConns,
			MaxIdleConnsPerHost: defaultMaxIdleConnsPerHost,
			MaxConnsPerHost:     defaultMaxConnsPerHost,
			IdleConnTimeout:     defaultIdleConnTimeout,
			ForceAttemptHTTP2:   true,
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
	}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestIngestUrlMustConform(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.IngestURL = "/collector"
	cfg.IngestKey = "1234-1234"

	assert.Error(t, cfg.Validate(), `"ingest_url" must contain a valid host`)
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.IngestURL = "https://example.com:8088/otel/ingest/rest"
	cfg.IngestKey = "1234-1234"

	params := exportertest.NewNopSettings(metadata.Type)
	_, err := createLogsExporter(t.Context(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateLogsNoConfig(t *testing.T) {
	params := exportertest.NewNopSettings(metadata.Type)
	_, err := createLogsExporter(t.Context(), params, nil)
	assert.Error(t, err)
}
