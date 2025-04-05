// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter/internal/metadata"
)

// NewFactory creates a factory for Alertmanager exporter
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "http://localhost:9093"
	clientConfig.Timeout = 30 * time.Second
	clientConfig.WriteBufferSize = 512 * 1024

	return &Config{
		GeneratorURL:    "opentelemetry-collector",
		DefaultSeverity: "info",
		EventLabels:     []string{"attr1", "attr2"},
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		BackoffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   exporterhelper.NewDefaultQueueConfig(),
		ClientConfig:    clientConfig,
	}
}

func createTracesExporter(ctx context.Context, set exporter.Settings, config component.Config) (exporter.Traces, error) {
	cfg := config.(*Config)

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf(
			"exporter config requires a non-empty \"endpoint\"")
	}
	return newTracesExporter(ctx, cfg, set)
}
