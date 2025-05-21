// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter/internal/metadata"
)

// NewFactory creates a factory for Logz.io exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = 30 * time.Second
	// Default to gzip compression
	clientConfig.Compression = configcompression.TypeGzip
	// We almost read 0 bytes, so no need to tune ReadBufferSize.
	clientConfig.WriteBufferSize = 512 * 1024
	return &Config{
		Region:        "",
		Token:         "",
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
		ClientConfig:  clientConfig,
	}
}

func getListenerURL(region, dataType string) string {
	var url string
	lowerCaseRegion := strings.ToLower(region)
	switch lowerCaseRegion {
	case "ca", "eu", "uk", "au":
		url = fmt.Sprintf("https://otlp-listener-%s.logz.io/v1/%s", lowerCaseRegion, dataType)
	default:
		url = fmt.Sprintf("https://otlp-listener.logz.io/v1/%s", dataType)
	}
	return url
}

func generateEndpoint(cfg *Config, dataType string) (string, error) {
	defaultURL := getListenerURL("", dataType)
	switch {
	case cfg.Endpoint != "":
		return cfg.Endpoint, nil
	case cfg.Region != "":
		return getListenerURL(cfg.Region, dataType), nil
	case cfg.Endpoint == "" && cfg.Region == "":
		return defaultURL, errors.New("failed to generate endpoint, Endpoint or Region must be set")
	default:
		return defaultURL, nil
	}
}

func createTracesExporter(_ context.Context, params exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	exporterConfig := cfg.(*Config)
	return newLogzioTracesExporter(exporterConfig, params)
}

func createLogsExporter(_ context.Context, params exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	exporterConfig := cfg.(*Config)
	return newLogzioLogsExporter(exporterConfig, params)
}
