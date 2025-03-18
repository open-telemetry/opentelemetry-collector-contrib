// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

var (
	errUnexpectedConfigurationType = errors.New("failed to cast configuration to Azure Monitor Config")
	exporters                      = sharedcomponent.NewSharedComponents()
)

// NewFactory returns a factory for Azure Monitor exporter.
func NewFactory() exporter.Factory {
	f := &factory{
		loggerInitOnce: sync.Once{},
	}
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(f.createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(f.createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(f.createMetricsExporter, metadata.MetricsStability))
}

// Implements the interface from go.opentelemetry.io/collector/exporter/factory.go
type factory struct {
	loggerInitOnce sync.Once
}

func createDefaultConfig() component.Config {
	return &Config{
		MaxBatchSize:        1024,
		MaxBatchInterval:    10 * time.Second,
		SpanEventsEnabled:   false,
		QueueSettings:       exporterhelper.NewDefaultQueueConfig(),
		ShutdownTimeout:     1 * time.Second,
		CustomEventsEnabled: false,
	}
}

func (f *factory) createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	f.initLogger(set.Logger)
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errUnexpectedConfigurationType
	}
	ame := getOrCreateAzureMonitorExporter(cfg, set)
	origComp := ame.Unwrap().(*azureMonitorExporter)

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		origComp.consumeTraces,
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithStart(ame.Start),
		exporterhelper.WithShutdown(ame.Shutdown))
}

func (f *factory) createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	f.initLogger(set.Logger)
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errUnexpectedConfigurationType
	}
	ame := getOrCreateAzureMonitorExporter(cfg, set)
	origComp := ame.Unwrap().(*azureMonitorExporter)

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		origComp.consumeLogs,
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithStart(ame.Start),
		exporterhelper.WithShutdown(ame.Shutdown))
}

func (f *factory) createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	f.initLogger(set.Logger)
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errUnexpectedConfigurationType
	}
	ame := getOrCreateAzureMonitorExporter(cfg, set)
	origComp := ame.Unwrap().(*azureMonitorExporter)

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		origComp.consumeMetrics,
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithStart(ame.Start),
		exporterhelper.WithShutdown(ame.Shutdown))
}

func getOrCreateAzureMonitorExporter(cfg component.Config, set exporter.Settings) *sharedcomponent.SharedComponent {
	conf := cfg.(*Config)
	ame := exporters.GetOrAdd(set.ID, func() component.Component {
		return &azureMonitorExporter{
			config: conf,
			logger: set.Logger,
			packer: newMetricPacker(set.Logger),
		}
	})

	return ame
}

func (f *factory) initLogger(logger *zap.Logger) {
	f.loggerInitOnce.Do(func() {
		if checkedEntry := logger.Check(zap.DebugLevel, ""); checkedEntry != nil {
			appinsights.NewDiagnosticsMessageListener(func(msg string) error {
				logger.Debug(msg)
				return nil
			})
		}
	})
}
