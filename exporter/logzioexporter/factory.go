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

func getTracesListenerURL(region string) string {
	var url string
	lowerCaseRegion := strings.ToLower(region)
	switch lowerCaseRegion {
	case "us":
		url = "https://listener.logz.io:8071"
	case "ca":
		url = "https://listener-ca.logz.io:8071"
	case "eu":
		url = "https://listener-eu.logz.io:8071"
	case "uk":
		url = "https://listener-uk.logz.io:8071"
	case "au":
		url = "https://listener-au.logz.io:8071"
	case "nl":
		url = "https://listener-nl.logz.io:8071"
	case "wa":
		url = "https://listener-wa.logz.io:8071"
	default:
		url = "https://listener.logz.io:8071"
	}
	return url
}

func getLogsListenerURL(region string) string {
	var url string
	lowerCaseRegion := strings.ToLower(region)
	switch lowerCaseRegion {
	case "us":
		url = "https://otlp-listener.logz.io/v1/logs"
	case "ca":
		url = "https://otlp-listener-ca.logz.io/v1/logs"
	case "eu":
		url = "https://otlp-listener-eu.logz.io/v1/logs"
	case "uk":
		url = "https://otlp-listener-uk.logz.io/v1/logs"
	case "au":
		url = "https://otlp-listener-au.logz.io/v1/logs"
	case "nl":
		url = "https://otlp-listener-nl.logz.io/v1/logs"
	case "wa":
		url = "https://otlp-listener-wa.logz.io/v1/logs"
	default:
		url = "https://otlp-listener.logz.io/v1/logs"
	}
	return url
}

func generateTracesEndpoint(cfg *Config) (string, error) {
	if cfg.ClientConfig.Endpoint != "" {
		return cfg.ClientConfig.Endpoint, nil
	}
	if cfg.Region != "" {
		return fmt.Sprintf("%s/?token=%s", getTracesListenerURL(cfg.Region), string(cfg.Token)), nil
	}
	return fmt.Sprintf("%s/?token=%s", getTracesListenerURL(""), string(cfg.Token)), errors.New("failed to generate endpoint, Endpoint or Region must be set")
}

func generateLogsEndpoint(cfg *Config) (string, error) {
	if cfg.ClientConfig.Endpoint != "" {
		return cfg.ClientConfig.Endpoint, nil
	}
	if cfg.Region != "" {
		return getLogsListenerURL(cfg.Region), nil
	}
	return getLogsListenerURL(""), errors.New("failed to generate endpoint, Endpoint or Region must be set")
}

func createTracesExporter(_ context.Context, params exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	exporterConfig := cfg.(*Config)
	return newLogzioTracesExporter(exporterConfig, params)
}

func createLogsExporter(_ context.Context, params exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	exporterConfig := cfg.(*Config)
	return newLogzioLogsExporter(exporterConfig, params)
}
