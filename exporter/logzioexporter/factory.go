// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr = "logzio"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for Logz.io exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
		exporter.WithLogs(createLogsExporter, component.StabilityLevelBeta))

}

func createDefaultConfig() component.Config {
	return &Config{
		Region:        "",
		Token:         "",
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "",
			Timeout:  30 * time.Second,
			Headers:  map[string]configopaque.String{},
			// Default to gzip compression
			Compression: configcompression.Gzip,
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			WriteBufferSize: 512 * 1024,
		},
	}
}

func getListenerURL(region string) string {
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

func generateEndpoint(cfg *Config) (string, error) {
	defaultURL := fmt.Sprintf("%s/?token=%s", getListenerURL(""), cfg.Token)
	switch {
	case cfg.HTTPClientSettings.Endpoint != "":
		return cfg.HTTPClientSettings.Endpoint, nil
	case cfg.Region != "":
		return fmt.Sprintf("%s/?token=%s", getListenerURL(cfg.Region), cfg.Token), nil
	case cfg.HTTPClientSettings.Endpoint == "" && cfg.Region == "":
		return defaultURL, errors.New("failed to generate endpoint, Endpoint or Region must be set")
	default:
		return defaultURL, nil
	}
}

func createTracesExporter(_ context.Context, params exporter.CreateSettings, cfg component.Config) (exporter.Traces, error) {
	exporterConfig := cfg.(*Config)
	return newLogzioTracesExporter(exporterConfig, params)
}

func createLogsExporter(_ context.Context, params exporter.CreateSettings, cfg component.Config) (exporter.Logs, error) {
	exporterConfig := cfg.(*Config)
	return newLogzioLogsExporter(exporterConfig, params)
}
