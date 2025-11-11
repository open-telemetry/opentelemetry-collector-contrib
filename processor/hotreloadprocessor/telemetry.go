// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"os"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/hotreloadprocessor/internal/metadata"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// hotreloadProcessorTelemetry holds telemetry data and methods for recording metrics.
type hotreloadProcessorTelemetry struct {
	exportCtx        context.Context
	processorAttr    []attribute.KeyValue
	telemetryBuilder *metadata.TelemetryBuilder
}

// trigger represents different types of telemetry triggers.
type trigger int

const (
	triggerProcessDuration trigger = iota
	triggerConfigRefreshInterval
	triggerConfigShutdownDelay
	triggerNewestFileSuccessTimestamp
	triggerNewestFileFailedTimestamp
	triggerRollbackFileSuccessTimestamp
	triggerRunningProcessorsCount
	triggerReloadDuration
	triggerScan
)

// newHotReloadProcessorTelemetry initializes a new hotreloadProcessorTelemetry instance.
func newHotReloadProcessorTelemetry(
	set processor.Settings,
	configurationFile string,
) (*hotreloadProcessorTelemetry, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	idParts := strings.Split(set.ID.String(), "/")
	collectorVersion := os.Getenv("COLLECTOR_VERSION")
	processorAttr := []attribute.KeyValue{
		attribute.String("processor", set.ID.String()),
		attribute.String("configuration_file", configurationFile),
		attribute.String("destination", idParts[len(idParts)-1]),
		attribute.String("collector_version", collectorVersion),
	}

	return &hotreloadProcessorTelemetry{
		processorAttr:    processorAttr,
		exportCtx:        context.Background(),
		telemetryBuilder: telemetryBuilder,
	}, nil
}

// record logs the telemetry data based on the trigger type.
func (pt *hotreloadProcessorTelemetry) record(
	trigger trigger,
	value float64,
	labels ...attribute.KeyValue,
) {
	metricFunc := pt.getMetricFuncForTrigger(trigger)
	allLabels := make([]attribute.KeyValue, 0, len(labels)+len(pt.processorAttr))
	allLabels = append(allLabels, labels...)
	allLabels = append(allLabels, pt.processorAttr...)
	if metricFunc != nil {
		metricFunc(pt, value, allLabels...)
	}
}

// metricRecordFunc defines the function signature for recording metrics.
type metricRecordFunc func(*hotreloadProcessorTelemetry, float64, ...attribute.KeyValue)

// getMetricFuncForTrigger returns the appropriate metric recording function based on the trigger type.
func (pt *hotreloadProcessorTelemetry) getMetricFuncForTrigger(
	triggerName trigger,
) metricRecordFunc {
	metricMap := map[trigger]metricRecordFunc{
		triggerProcessDuration: recordInt64Histogram(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Histogram {
				return pt.telemetryBuilder.ProcessorHotReloadProcessDuration
			},
		),
		triggerConfigRefreshInterval: recordInt64Gauge(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Gauge {
				return pt.telemetryBuilder.ProcessorHotReloadConfigRefreshInterval
			},
		),
		triggerConfigShutdownDelay: recordInt64Gauge(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Gauge {
				return pt.telemetryBuilder.ProcessorHotReloadConfigShutdownDelay
			},
		),
		triggerNewestFileSuccessTimestamp: recordInt64Gauge(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Gauge {
				return pt.telemetryBuilder.ProcessorHotReloadNewestFileSuccessTimestamp
			},
		),
		triggerNewestFileFailedTimestamp: recordInt64Gauge(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Gauge {
				return pt.telemetryBuilder.ProcessorHotReloadNewestFileFailedTimestamp
			},
		),
		triggerRollbackFileSuccessTimestamp: recordInt64Gauge(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Gauge {
				return pt.telemetryBuilder.ProcessorHotReloadRollbackFileSuccessTimestamp
			},
		),
		triggerRunningProcessorsCount: recordInt64Gauge(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Gauge {
				return pt.telemetryBuilder.ProcessorHotReloadRunningProcessorsCount
			},
		),
		triggerReloadDuration: recordInt64Histogram(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Histogram {
				return pt.telemetryBuilder.ProcessorHotReloadReloadDuration
			},
		),
		triggerScan: recordInt64Gauge(
			func(pt *hotreloadProcessorTelemetry) metric.Int64Gauge {
				return pt.telemetryBuilder.ProcessorHotReloadScan
			},
		),
	}
	return metricMap[triggerName]
}

// recordInt64Counter returns a function to record int64 counter metrics.
func recordInt64Counter(
	getMetric func(*hotreloadProcessorTelemetry) metric.Int64Counter,
) metricRecordFunc {
	return func(qpt *hotreloadProcessorTelemetry, value float64, labels ...attribute.KeyValue) {
		getMetric(qpt).Add(qpt.exportCtx, int64(value), metric.WithAttributes(labels...))
	}
}

// recordInt64Gauge returns a function to record int64 gauge metrics.
func recordInt64Gauge(
	getMetric func(*hotreloadProcessorTelemetry) metric.Int64Gauge,
) metricRecordFunc {
	return func(pt *hotreloadProcessorTelemetry, value float64, labels ...attribute.KeyValue) {
		getMetric(pt).Record(pt.exportCtx, int64(value), metric.WithAttributes(labels...))
	}
}

// recordInt64Histogram returns a function to record int64 histogram metrics.
func recordInt64Histogram(
	getMetric func(*hotreloadProcessorTelemetry) metric.Int64Histogram,
) metricRecordFunc {
	return func(pt *hotreloadProcessorTelemetry, value float64, labels ...attribute.KeyValue) {
		getMetric(pt).Record(pt.exportCtx, int64(value), metric.WithAttributes(labels...))
	}
}
