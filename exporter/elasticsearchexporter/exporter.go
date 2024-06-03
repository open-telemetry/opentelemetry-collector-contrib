// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type elasticsearchExporter struct {
	component.TelemetrySettings
	userAgent string

	config           *Config
	index            string
	logstashFormat   LogstashFormatSettings
	dynamicIndex     bool
	dynamicIndexMode DynamicIndexMode
	model            mappingModel

	bulkIndexer *esBulkIndexerCurrent
}

func newExporter(
	cfg *Config,
	set exporter.Settings,
	index string,
	dynamicIndex bool,
	dynamicIndexMode DynamicIndexMode,
) (*elasticsearchExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	model := &encodeModel{
		dedup: cfg.Mapping.Dedup,
		dedot: cfg.Mapping.Dedot,
		mode:  cfg.MappingMode(),
	}

	userAgent := fmt.Sprintf(
		"%s/%s (%s/%s)",
		set.BuildInfo.Description,
		set.BuildInfo.Version,
		runtime.GOOS,
		runtime.GOARCH,
	)

	return &elasticsearchExporter{
		TelemetrySettings: set.TelemetrySettings,
		userAgent:         userAgent,

		config:           cfg,
		index:            index,
		dynamicIndex:     dynamicIndex,
		dynamicIndexMode: dynamicIndexMode,
		model:            model,
		logstashFormat:   cfg.LogstashFormat,
	}, nil
}

func (e *elasticsearchExporter) Start(ctx context.Context, host component.Host) error {
	client, err := newElasticsearchClient(ctx, e.config, host, e.TelemetrySettings, e.userAgent)
	if err != nil {
		return err
	}
	bulkIndexer, err := newBulkIndexer(e.Logger, client, e.config)
	if err != nil {
		return err
	}
	e.bulkIndexer = bulkIndexer
	return nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	if e.bulkIndexer != nil {
		return e.bulkIndexer.Close(ctx)
	}
	return nil
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	var errs []error

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)
			scope := ill.Scope()
			logs := ill.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				if err := e.pushLogRecord(ctx, resource, logs.At(k), scope); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					errs = append(errs, err)
				}
			}
		}
	}

	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushLogRecord(ctx context.Context, resource pcommon.Resource, record plog.LogRecord, scope pcommon.InstrumentationScope) error {
	fIndex := e.index
	if e.dynamicIndex {
		if e.dynamicIndexMode == DynamicIndexModeDataStream {
			dataSet := getFromAttributesNew(dataStreamDataset, defaultDataStreamDataset, record.Attributes(), scope.Attributes(), resource.Attributes())
			namespace := getFromAttributesNew(dataStreamNamespace, defaultDataStreamNamespace, record.Attributes(), scope.Attributes(), resource.Attributes())

			fIndex = fmt.Sprintf("logs-%s-%s", dataSet, namespace)
		} else {
			prefix := getFromAttributes(indexPrefix, resource, scope, record)
			suffix := getFromAttributes(indexSuffix, resource, scope, record)

			fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
		}
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeLog(resource, record, scope)
	if err != nil {
		return fmt.Errorf("failed to encode log event: %w", err)
	}
	return pushDocuments(ctx, fIndex, document, e.bulkIndexer)
}

func (e *elasticsearchExporter) pushMetricsData(
	ctx context.Context,
	metrics pmetric.Metrics,
) error {
	var errs []error

	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceMetric := resourceMetrics.At(i)
		resource := resourceMetric.Resource()
		scopeMetrics := resourceMetric.ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			scopeMetrics := scopeMetrics.At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)

				// We only support Sum and Gauge metrics at the moment.
				var dataPoints pmetric.NumberDataPointSlice
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					dataPoints = metric.Sum().DataPoints()
				case pmetric.MetricTypeGauge:
					dataPoints = metric.Gauge().DataPoints()
				}

				for l := 0; l < dataPoints.Len(); l++ {
					dataPoint := dataPoints.At(l)
					if err := e.pushMetricDataPoint(ctx, resource, scopeMetrics.Scope(), metric, dataPoint); err != nil {
						if cerr := ctx.Err(); cerr != nil {
							return cerr
						}
						errs = append(errs, err)
					}
				}
			}
		}
	}

	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushMetricDataPoint(
	ctx context.Context,
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	metric pmetric.Metric,
	dataPoint pmetric.NumberDataPoint,
) error {
	fIndex := e.index
	if e.dynamicIndex {
		if e.dynamicIndexMode == DynamicIndexModeDataStream {
			dataSet := getFromAttributesNew(dataStreamDataset, defaultDataStreamDataset, dataPoint.Attributes(), scope.Attributes(), resource.Attributes())
			namespace := getFromAttributesNew(dataStreamNamespace, defaultDataStreamNamespace, dataPoint.Attributes(), scope.Attributes(), resource.Attributes())
			fIndex = fmt.Sprintf("metrics-%s-%s", dataSet, namespace)
		} else {
			prefix := getFromAttributesNew(indexPrefix, "", dataPoint.Attributes(), scope.Attributes(), resource.Attributes())
			suffix := getFromAttributesNew(indexSuffix, "", dataPoint.Attributes(), scope.Attributes(), resource.Attributes())
			fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
		}
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeMetricDataPoint(resource, scope, metric, dataPoint)
	if err != nil {
		return fmt.Errorf("failed to encode a metric data point: %w", err)
	}

	return pushDocuments(ctx, fIndex, document, e.bulkIndexer)
}

func (e *elasticsearchExporter) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {
	var errs []error
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			scope := scopeSpan.Scope()
			spans := scopeSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if err := e.pushTraceRecord(ctx, resource, span, scope); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
			}
		}
	}

	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushTraceRecord(ctx context.Context, resource pcommon.Resource, span ptrace.Span, scope pcommon.InstrumentationScope) error {
	fIndex := e.index
	if e.dynamicIndex {
		if e.dynamicIndexMode == DynamicIndexModeDataStream {
			dataSet := getFromAttributesNew(dataStreamDataset, defaultDataStreamDataset, span.Attributes(), scope.Attributes(), resource.Attributes())
			namespace := getFromAttributesNew(dataStreamNamespace, defaultDataStreamNamespace, span.Attributes(), scope.Attributes(), resource.Attributes())

			fIndex = fmt.Sprintf("traces-%s-%s", dataSet, namespace)
		} else {
			prefix := getFromAttributes(indexPrefix, resource, scope, span)
			suffix := getFromAttributes(indexSuffix, resource, scope, span)

			fIndex = fmt.Sprintf("%s%s%s", prefix, fIndex, suffix)
		}
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeSpan(resource, span, scope)
	if err != nil {
		return fmt.Errorf("failed to encode trace record: %w", err)
	}
	return pushDocuments(ctx, fIndex, document, e.bulkIndexer)
}
