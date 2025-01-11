// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/pool"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type elasticsearchExporter struct {
	component.TelemetrySettings
	userAgent string

	config         *Config
	index          string
	logstashFormat LogstashFormatSettings
	dynamicIndex   bool
	model          mappingModel
	otel           bool

	wg          sync.WaitGroup // active sessions
	bulkIndexer bulkIndexer

	bufferPool *pool.BufferPool
}

func newExporter(
	cfg *Config,
	set exporter.Settings,
	index string,
	dynamicIndex bool,
) *elasticsearchExporter {
	model := &encodeModel{
		dedot: cfg.Mapping.Dedot,
		mode:  cfg.MappingMode(),
	}

	otel := model.mode == MappingOTel

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

		config:         cfg,
		index:          index,
		dynamicIndex:   dynamicIndex,
		model:          model,
		logstashFormat: cfg.LogstashFormat,
		otel:           otel,
		bufferPool:     pool.NewBufferPool(),
	}
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
		if err := e.bulkIndexer.Close(ctx); err != nil {
			return err
		}
	}

	doneCh := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		return nil
	}
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	e.wg.Add(1)
	defer e.wg.Done()

	session, err := e.bulkIndexer.StartSession(ctx)
	if err != nil {
		return err
	}
	defer session.End()

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
				if err := e.pushLogRecord(ctx, resource, rl.SchemaUrl(), logs.At(k), scope, ill.SchemaUrl(), session); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					if errors.Is(err, ErrInvalidTypeForBodyMapMode) {
						e.Logger.Warn("dropping log record", zap.Error(err))
						continue
					}

					errs = append(errs, err)
				}
			}
		}
	}

	if err := session.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushLogRecord(
	ctx context.Context,
	resource pcommon.Resource,
	resourceSchemaURL string,
	record plog.LogRecord,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	bulkIndexerSession bulkIndexerSession,
) error {
	fIndex := e.index
	if e.dynamicIndex {
		fIndex = routeLogRecord(record.Attributes(), scope.Attributes(), resource.Attributes(), fIndex, e.otel, scope.Name())
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	buffer := e.bufferPool.NewPooledBuffer()
	err := e.model.encodeLog(resource, resourceSchemaURL, record, scope, scopeSchemaURL, buffer.Buffer)
	if err != nil {
		return fmt.Errorf("failed to encode log event: %w", err)
	}
	return bulkIndexerSession.Add(ctx, fIndex, buffer, nil)
}

func (e *elasticsearchExporter) pushMetricsData(
	ctx context.Context,
	metrics pmetric.Metrics,
) error {
	e.wg.Add(1)
	defer e.wg.Done()

	session, err := e.bulkIndexer.StartSession(ctx)
	if err != nil {
		return err
	}
	defer session.End()

	var errs []error
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceMetric := resourceMetrics.At(i)
		resource := resourceMetric.Resource()
		scopeMetrics := resourceMetric.ScopeMetrics()

		for j := 0; j < scopeMetrics.Len(); j++ {
			var validationErrs []error // log instead of returning these so that upstream does not retry
			scopeMetrics := scopeMetrics.At(j)
			scope := scopeMetrics.Scope()
			groupedDataPointsByIndex := make(map[string]map[uint32][]dataPoint)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)

				upsertDataPoint := func(dp dataPoint) error {
					fIndex, err := e.getMetricDataPointIndex(resource, scope, dp)
					if err != nil {
						return err
					}
					groupedDataPoints, ok := groupedDataPointsByIndex[fIndex]
					if !ok {
						groupedDataPoints = make(map[uint32][]dataPoint)
						groupedDataPointsByIndex[fIndex] = groupedDataPoints
					}
					dpHash := e.model.hashDataPoint(dp)
					dataPoints, ok := groupedDataPoints[dpHash]
					if !ok {
						groupedDataPoints[dpHash] = []dataPoint{dp}
					} else {
						groupedDataPoints[dpHash] = append(dataPoints, dp)
					}
					return nil
				}

				switch metric.Type() {
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if err := upsertDataPoint(newNumberDataPoint(metric, dp)); err != nil {
							validationErrs = append(validationErrs, err)
							continue
						}
					}
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if err := upsertDataPoint(newNumberDataPoint(metric, dp)); err != nil {
							validationErrs = append(validationErrs, err)
							continue
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					if metric.ExponentialHistogram().AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
						validationErrs = append(validationErrs, fmt.Errorf("dropping cumulative temporality exponential histogram %q", metric.Name()))
						continue
					}
					dps := metric.ExponentialHistogram().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if err := upsertDataPoint(newExponentialHistogramDataPoint(metric, dp)); err != nil {
							validationErrs = append(validationErrs, err)
							continue
						}
					}
				case pmetric.MetricTypeHistogram:
					if metric.Histogram().AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
						validationErrs = append(validationErrs, fmt.Errorf("dropping cumulative temporality histogram %q", metric.Name()))
						continue
					}
					dps := metric.Histogram().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if err := upsertDataPoint(newHistogramDataPoint(metric, dp)); err != nil {
							validationErrs = append(validationErrs, err)
							continue
						}
					}
				case pmetric.MetricTypeSummary:
					dps := metric.Summary().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if err := upsertDataPoint(newSummaryDataPoint(metric, dp)); err != nil {
							validationErrs = append(validationErrs, err)
							continue
						}
					}
				}
			}

			for fIndex, groupedDataPoints := range groupedDataPointsByIndex {
				for _, dataPoints := range groupedDataPoints {
					buf := e.bufferPool.NewPooledBuffer()
					dynamicTemplates, err := e.model.encodeMetrics(resource, resourceMetric.SchemaUrl(), scope, scopeMetrics.SchemaUrl(), dataPoints, &validationErrs, buf.Buffer)
					if err != nil {
						errs = append(errs, err)
						continue
					}
					if err := session.Add(ctx, fIndex, buf, dynamicTemplates); err != nil {
						if cerr := ctx.Err(); cerr != nil {
							return cerr
						}
						errs = append(errs, err)
					}
				}
			}
			if len(validationErrs) > 0 {
				e.Logger.Warn("validation errors", zap.Error(errors.Join(validationErrs...)))
			}
		}
	}

	if err := session.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) getMetricDataPointIndex(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	dataPoint dataPoint,
) (string, error) {
	fIndex := e.index
	if e.dynamicIndex {
		fIndex = routeDataPoint(dataPoint.Attributes(), scope.Attributes(), resource.Attributes(), fIndex, e.otel, scope.Name())
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return "", err
		}
		fIndex = formattedIndex
	}
	return fIndex, nil
}

func (e *elasticsearchExporter) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {
	e.wg.Add(1)
	defer e.wg.Done()

	session, err := e.bulkIndexer.StartSession(ctx)
	if err != nil {
		return err
	}
	defer session.End()

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
				if err := e.pushTraceRecord(ctx, resource, il.SchemaUrl(), span, scope, scopeSpan.SchemaUrl(), session); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
				for ii := 0; ii < span.Events().Len(); ii++ {
					spanEvent := span.Events().At(ii)
					if err := e.pushSpanEvent(ctx, resource, il.SchemaUrl(), span, spanEvent, scope, scopeSpan.SchemaUrl(), session); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
	}

	if err := session.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushTraceRecord(
	ctx context.Context,
	resource pcommon.Resource,
	resourceSchemaURL string,
	span ptrace.Span,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	bulkIndexerSession bulkIndexerSession,
) error {
	fIndex := e.index
	if e.dynamicIndex {
		fIndex = routeSpan(span.Attributes(), scope.Attributes(), resource.Attributes(), fIndex, e.otel, span.Name())
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	buf := e.bufferPool.NewPooledBuffer()
	err := e.model.encodeSpan(resource, resourceSchemaURL, span, scope, scopeSchemaURL, buf.Buffer)
	if err != nil {
		return fmt.Errorf("failed to encode trace record: %w", err)
	}
	return bulkIndexerSession.Add(ctx, fIndex, buf, nil)
}

func (e *elasticsearchExporter) pushSpanEvent(
	ctx context.Context,
	resource pcommon.Resource,
	resourceSchemaURL string,
	span ptrace.Span,
	spanEvent ptrace.SpanEvent,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	bulkIndexerSession bulkIndexerSession,
) error {
	fIndex := e.index
	if e.dynamicIndex {
		fIndex = routeSpanEvent(spanEvent.Attributes(), scope.Attributes(), resource.Attributes(), fIndex, e.otel, scope.Name())
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}
	buf := e.bufferPool.NewPooledBuffer()
	e.model.encodeSpanEvent(resource, resourceSchemaURL, span, spanEvent, scope, scopeSchemaURL, buf.Buffer)
	if buf.Buffer.Len() == 0 {
		return nil
	}

	return bulkIndexerSession.Add(ctx, fIndex, buf, nil)
}
