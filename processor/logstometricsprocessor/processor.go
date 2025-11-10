// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logstometricsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/internal/aggregator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/internal/customottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/internal/model"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type logsToMetricsProcessor struct {
	config           *config.Config
	nextLogsConsumer consumer.Logs
	metricsExporter  exporter.Metrics
	logger           *zap.Logger
	telemetryBuilder *metadata.TelemetryBuilder
	collectorInfo    model.CollectorInstanceInfo
	logMetricDefs    []model.MetricDef[ottllog.TransformContext]
	dropLogs         bool
}

func newProcessor(
	cfg *config.Config,
	nextLogsConsumer consumer.Logs,
	settings processor.Settings,
) (*logsToMetricsProcessor, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry builder: %w", err)
	}

	parser, err := ottllog.NewParser(customottl.LogFuncs(), settings.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTTL statement parser for logs: %w", err)
	}

	metricDefs := make([]model.MetricDef[ottllog.TransformContext], 0, len(cfg.Logs))
	for i := range cfg.Logs {
		info := cfg.Logs[i]
		var md model.MetricDef[ottllog.TransformContext]
		if err := md.FromMetricInfo(info, parser, settings.TelemetrySettings); err != nil {
			return nil, fmt.Errorf("failed to parse provided metric information: %w", err)
		}
		metricDefs = append(metricDefs, md)
	}

	return &logsToMetricsProcessor{
		config:           cfg,
		logger:           settings.Logger,
		telemetryBuilder: telemetryBuilder,
		collectorInfo: model.NewCollectorInstanceInfo(
			settings.TelemetrySettings,
		),
		nextLogsConsumer: nextLogsConsumer,
		logMetricDefs:    metricDefs,
		dropLogs:         cfg.DropLogs,
	}, nil
}

// DataType represents the type of telemetry data
type DataType int

const (
	// DataTypeMetrics represents metrics data type
	DataTypeMetrics DataType = iota
)

// exporterHost is an interface for accessing exporters from component.Host
type exporterHost interface {
	GetExporters() map[DataType]map[component.ID]component.Component
}

func (p *logsToMetricsProcessor) Start(ctx context.Context, host component.Host) error {
	eh, ok := host.(exporterHost)
	if !ok {
		return fmt.Errorf("host does not support GetExporters() method")
	}

	exporters := eh.GetExporters()
	metricsExporters, ok := exporters[DataTypeMetrics]
	if !ok {
		return fmt.Errorf("no metrics exporters available")
	}

	exp, ok := metricsExporters[p.config.MetricsExporter]
	if !ok {
		return fmt.Errorf("metrics exporter %s not found", p.config.MetricsExporter)
	}

	metricsExporter, ok := exp.(exporter.Metrics)
	if !ok {
		return fmt.Errorf("exporter %s is not a metrics exporter", p.config.MetricsExporter)
	}

	p.metricsExporter = metricsExporter
	return nil
}

func (p *logsToMetricsProcessor) Shutdown(context.Context) error {
	if p.telemetryBuilder != nil {
		p.telemetryBuilder.Shutdown()
	}
	return nil
}

func (p *logsToMetricsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// processLogs implements the ProcessLogsFunc type for processorhelper.NewLogs
func (p *logsToMetricsProcessor) processLogs(ctx context.Context, logs plog.Logs) (plog.Logs, error) {
	err := p.ConsumeLogs(ctx, logs)
	if err != nil {
		return logs, err
	}
	// If logs were dropped, return error to skip forwarding (similar to probabilistic sampler)
	// Otherwise return original logs for forwarding
	if p.dropLogs {
		return logs, processorhelper.ErrSkipProcessingData
	}
	return logs, nil
}

func (p *logsToMetricsProcessor) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	startTime := time.Now()
	logCount := logs.LogRecordCount()
	var errorCount int64
	var metricsExtracted int64

	if len(p.logMetricDefs) == 0 {
		// No metric definitions, just record telemetry
		if p.dropLogs {
			p.telemetryBuilder.RecordProcessorLogstometricsLogsDropped(ctx, int64(logCount))
		}
		return nil
	}

	processedMetrics := pmetric.NewMetrics()
	processedMetrics.ResourceMetrics().EnsureCapacity(logs.ResourceLogs().Len())
	agg := aggregator.NewAggregator[ottllog.TransformContext](processedMetrics)

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLog := logs.ResourceLogs().At(i)
		resourceAttrs := resourceLog.Resource().Attributes()
		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeLog := resourceLog.ScopeLogs().At(j)
			for k := 0; k < scopeLog.LogRecords().Len(); k++ {
				log := scopeLog.LogRecords().At(k)
				logAttrs := log.Attributes()
				for _, md := range p.logMetricDefs {
					filteredLogAttrs, ok := md.FilterAttributes(logAttrs)
					if !ok {
						continue
					}

					// The transform context is created from original attributes so that the
					// OTTL expressions are also applied on the original attributes.
					tCtx := ottllog.NewTransformContext(log, scopeLog.Scope(), resourceLog.Resource(), scopeLog, resourceLog)
					if md.Conditions != nil {
						match, err := md.Conditions.Eval(ctx, tCtx)
						if err != nil {
							p.logger.Debug("condition evaluation error", zap.String("name", md.Key.Name), zap.Error(err))
							errorCount++
							continue
						}
						if !match {
							p.logger.Debug("condition not matched, skipping", zap.String("name", md.Key.Name))
							continue
						}
					}
					filteredResAttrs := md.FilterResourceAttributes(resourceAttrs, p.collectorInfo)
					if err := agg.Aggregate(ctx, tCtx, md, filteredResAttrs, filteredLogAttrs, 1); err != nil {
						p.logger.Debug("aggregation error", zap.String("name", md.Key.Name), zap.Error(err))
						errorCount++
						continue
					}
					metricsExtracted++
				}
			}
		}
	}
	agg.Finalize(p.logMetricDefs)

	// Send metrics to exporter
	if processedMetrics.ResourceMetrics().Len() > 0 {
		if err := p.metricsExporter.ConsumeMetrics(ctx, processedMetrics); err != nil {
			p.logger.Error("Failed to export metrics", zap.Error(err))
			errorCount++
			// Continue processing logs even if metrics export fails
		}
	}

	// Record telemetry
	p.telemetryBuilder.RecordProcessorLogstometricsLogsProcessed(ctx, int64(logCount))
	p.telemetryBuilder.RecordProcessorLogstometricsMetricsExtracted(ctx, metricsExtracted)
	if errorCount > 0 {
		p.telemetryBuilder.RecordProcessorLogstometricsErrors(ctx, errorCount)
	}

	// Record processing duration
	duration := time.Since(startTime)
	p.telemetryBuilder.RecordProcessorLogstometricsProcessingDuration(ctx, duration.Milliseconds())

	// Don't forward logs here - processLogs will handle forwarding/dropping
	// Just record telemetry for dropped logs
	if p.dropLogs {
		p.telemetryBuilder.RecordProcessorLogstometricsLogsDropped(ctx, int64(logCount))
	}
	return nil
}

