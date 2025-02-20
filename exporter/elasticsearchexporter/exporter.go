// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/elastic/go-docappender/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/pool"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"
)

type elasticsearchExporter struct {
	component.TelemetrySettings
	userAgent string

	config       *Config
	index        string
	dynamicIndex bool
	model        mappingModel

	wg          sync.WaitGroup // active sessions
	bulkIndexer bulkIndexer

	// Profiles requires multiple bulk indexers depending on the data type
	// Bulk indexer for profiling-events-*
	biEvents bulkIndexer
	// Bulk indexer for profiling-stacktraces
	biStackTraces bulkIndexer
	// Bulk indexer for profiling-stackframes
	biStackFrames bulkIndexer
	// Bulk indexer for profiling-executables
	biExecutables bulkIndexer

	bufferPool *pool.BufferPool
}

func newExporter(
	cfg *Config,
	set exporter.Settings,
	index string,
	dynamicIndex bool,
) *elasticsearchExporter {
	model := &encodeModel{
		mode: cfg.MappingMode(),
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

		config:       cfg,
		index:        index,
		dynamicIndex: dynamicIndex,
		model:        model,
		bufferPool:   pool.NewBufferPool(),
	}
}

func (e *elasticsearchExporter) Start(ctx context.Context, host component.Host) error {
	client, err := newElasticsearchClient(ctx, e.config, host, e.TelemetrySettings, e.userAgent)
	if err != nil {
		return err
	}
	bulkIndexer, err := newBulkIndexer(e.Logger, client, e.config, e.config.MappingMode() == MappingOTel)
	if err != nil {
		return err
	}
	e.bulkIndexer = bulkIndexer
	biEvents, err := newBulkIndexer(e.Logger, client, e.config, true)
	if err != nil {
		return err
	}
	e.biEvents = biEvents
	biStackTraces, err := newBulkIndexer(e.Logger, client, e.config, false)
	if err != nil {
		return err
	}
	e.biStackTraces = biStackTraces
	biStackFrames, err := newBulkIndexer(e.Logger, client, e.config, false)
	if err != nil {
		return err
	}
	e.biStackFrames = biStackFrames
	biExecutables, err := newBulkIndexer(e.Logger, client, e.config, false)
	if err != nil {
		return err
	}
	e.biExecutables = biExecutables

	return nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	if e.bulkIndexer != nil {
		if err := e.bulkIndexer.Close(ctx); err != nil {
			return err
		}
	}
	if e.biEvents != nil {
		if err := e.biEvents.Close(ctx); err != nil {
			return err
		}
	}
	if e.biStackTraces != nil {
		if err := e.biStackTraces.Close(ctx); err != nil {
			return err
		}
	}
	if e.biStackFrames != nil {
		if err := e.biStackFrames.Close(ctx); err != nil {
			return err
		}
	}
	if e.biExecutables != nil {
		if err := e.biExecutables.Close(ctx); err != nil {
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
	mappingMode := e.config.MappingMode()
	router := newDocumentRouter(mappingMode, e.dynamicIndex, e.index, e.config)

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
				if err := e.pushLogRecord(ctx, router, resource, rl.SchemaUrl(), logs.At(k), scope, ill.SchemaUrl(), session); err != nil {
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
	router documentRouter,
	resource pcommon.Resource,
	resourceSchemaURL string,
	record plog.LogRecord,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	bulkIndexerSession bulkIndexerSession,
) error {
	index, err := router.routeLogRecord(resource, scope, record.Attributes())
	if err != nil {
		return err
	}

	buf := e.bufferPool.NewPooledBuffer()
	docID := e.extractDocumentIDAttribute(record.Attributes())
	if err := e.model.encodeLog(resource, resourceSchemaURL, record, scope, scopeSchemaURL, index, buf.Buffer); err != nil {
		buf.Recycle()
		return fmt.Errorf("failed to encode log event: %w", err)
	}

	// not recycling after Add returns an error as we don't know if it's already recycled
	return bulkIndexerSession.Add(ctx, index.Index, docID, buf, nil, docappender.ActionCreate)
}

type dataPointsGroup struct {
	resource          pcommon.Resource
	resourceSchemaURL string
	scope             pcommon.InstrumentationScope
	scopeSchemaURL    string
	dataPoints        []datapoints.DataPoint
}

func (p *dataPointsGroup) addDataPoint(dp datapoints.DataPoint) {
	p.dataPoints = append(p.dataPoints, dp)
}

func (e *elasticsearchExporter) pushMetricsData(
	ctx context.Context,
	metrics pmetric.Metrics,
) error {
	mappingMode := e.config.MappingMode()
	router := newDocumentRouter(mappingMode, e.dynamicIndex, e.index, e.config)
	hasher := newDataPointHasher(mappingMode)

	e.wg.Add(1)
	defer e.wg.Done()

	session, err := e.bulkIndexer.StartSession(ctx)
	if err != nil {
		return err
	}
	defer session.End()

	groupedDataPointsByIndex := make(map[elasticsearch.Index]map[uint32]*dataPointsGroup)
	var validationErrs []error // log instead of returning these so that upstream does not retry
	var errs []error
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceMetric := resourceMetrics.At(i)
		resource := resourceMetric.Resource()
		scopeMetrics := resourceMetric.ScopeMetrics()

		for j := 0; j < scopeMetrics.Len(); j++ {
			scopeMetrics := scopeMetrics.At(j)
			scope := scopeMetrics.Scope()
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)

				upsertDataPoint := func(dp datapoints.DataPoint) error {
					index, err := router.routeDataPoint(resource, scope, dp.Attributes())
					if err != nil {
						return err
					}
					groupedDataPoints, ok := groupedDataPointsByIndex[index]
					if !ok {
						groupedDataPoints = make(map[uint32]*dataPointsGroup)
						groupedDataPointsByIndex[index] = groupedDataPoints
					}
					dpHash := hasher.hashDataPoint(resource, scope, dp)
					dpGroup, ok := groupedDataPoints[dpHash]
					if !ok {
						groupedDataPoints[dpHash] = &dataPointsGroup{
							resource:   resource,
							scope:      scope,
							dataPoints: []datapoints.DataPoint{dp},
						}
					} else {
						dpGroup.addDataPoint(dp)
					}
					return nil
				}

				switch metric.Type() {
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if err := upsertDataPoint(datapoints.NewNumber(metric, dp)); err != nil {
							validationErrs = append(validationErrs, err)
							continue
						}
					}
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if err := upsertDataPoint(datapoints.NewNumber(metric, dp)); err != nil {
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
						if err := upsertDataPoint(datapoints.NewExponentialHistogram(metric, dp)); err != nil {
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
						if err := upsertDataPoint(datapoints.NewHistogram(metric, dp)); err != nil {
							validationErrs = append(validationErrs, err)
							continue
						}
					}
				case pmetric.MetricTypeSummary:
					dps := metric.Summary().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						if err := upsertDataPoint(datapoints.NewSummary(metric, dp)); err != nil {
							validationErrs = append(validationErrs, err)
							continue
						}
					}
				}
			}
		}
	}

	for index, groupedDataPoints := range groupedDataPointsByIndex {
		for _, dpGroup := range groupedDataPoints {
			buf := e.bufferPool.NewPooledBuffer()
			dynamicTemplates, err := e.model.encodeMetrics(
				dpGroup.resource, dpGroup.resourceSchemaURL,
				dpGroup.scope, dpGroup.scopeSchemaURL,
				dpGroup.dataPoints, &validationErrs, index, buf.Buffer,
			)
			if err != nil {
				buf.Recycle()
				errs = append(errs, err)
				continue
			}
			if err := session.Add(ctx, index.Index, "", buf, dynamicTemplates, docappender.ActionCreate); err != nil {
				// not recycling after Add returns an error as we don't know if it's already recycled
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

	if err := session.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {
	mappingMode := e.config.MappingMode()
	router := newDocumentRouter(mappingMode, e.dynamicIndex, e.index, e.config)

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
				if err := e.pushTraceRecord(ctx, router, resource, il.SchemaUrl(), span, scope, scopeSpan.SchemaUrl(), session); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
				for ii := 0; ii < span.Events().Len(); ii++ {
					spanEvent := span.Events().At(ii)
					if err := e.pushSpanEvent(ctx, router, resource, il.SchemaUrl(), span, spanEvent, scope, scopeSpan.SchemaUrl(), session); err != nil {
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
	router documentRouter,
	resource pcommon.Resource,
	resourceSchemaURL string,
	span ptrace.Span,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	bulkIndexerSession bulkIndexerSession,
) error {
	index, err := router.routeSpan(resource, scope, span.Attributes())
	if err != nil {
		return err
	}

	buf := e.bufferPool.NewPooledBuffer()
	if err := e.model.encodeSpan(resource, resourceSchemaURL, span, scope, scopeSchemaURL, index, buf.Buffer); err != nil {
		buf.Recycle()
		return fmt.Errorf("failed to encode trace record: %w", err)
	}
	// not recycling after Add returns an error as we don't know if it's already recycled
	return bulkIndexerSession.Add(ctx, index.Index, "", buf, nil, docappender.ActionCreate)
}

func (e *elasticsearchExporter) pushSpanEvent(
	ctx context.Context,
	router documentRouter,
	resource pcommon.Resource,
	resourceSchemaURL string,
	span ptrace.Span,
	spanEvent ptrace.SpanEvent,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	bulkIndexerSession bulkIndexerSession,
) error {
	index, err := router.routeSpanEvent(resource, scope, spanEvent.Attributes())
	if err != nil {
		return err
	}

	buf := e.bufferPool.NewPooledBuffer()
	e.model.encodeSpanEvent(resource, resourceSchemaURL, span, spanEvent, scope, scopeSchemaURL, index, buf.Buffer)
	if buf.Buffer.Len() == 0 {
		buf.Recycle()
		return nil
	}
	// not recycling after Add returns an error as we don't know if it's already recycled
	return bulkIndexerSession.Add(ctx, index.Index, "", buf, nil, docappender.ActionCreate)
}

func (e *elasticsearchExporter) extractDocumentIDAttribute(m pcommon.Map) string {
	if !e.config.LogsDynamicID.Enabled {
		return ""
	}

	v, ok := m.Get(elasticsearch.DocumentIDAttributeName)
	if !ok {
		return ""
	}
	return v.AsString()
}

func (e *elasticsearchExporter) pushProfilesData(ctx context.Context, pd pprofile.Profiles) error {
	e.wg.Add(1)
	defer e.wg.Done()

	defaultSession, err := e.bulkIndexer.StartSession(ctx)
	if err != nil {
		return err
	}
	defer defaultSession.End()
	eventsSession, err := e.biEvents.StartSession(ctx)
	if err != nil {
		return err
	}
	defer eventsSession.End()
	stackTracesSession, err := e.biStackTraces.StartSession(ctx)
	if err != nil {
		return err
	}
	defer stackTracesSession.End()
	stackFramesSession, err := e.biStackFrames.StartSession(ctx)
	if err != nil {
		return err
	}
	defer stackFramesSession.End()
	executablesSession, err := e.biExecutables.StartSession(ctx)
	if err != nil {
		return err
	}
	defer executablesSession.End()

	var errs []error
	rps := pd.ResourceProfiles()
	for i := 0; i < rps.Len(); i++ {
		rp := rps.At(i)
		resource := rp.Resource()
		sps := rp.ScopeProfiles()
		for j := 0; j < sps.Len(); j++ {
			sp := sps.At(j)
			scope := sp.Scope()
			p := sp.Profiles()
			for k := 0; k < p.Len(); k++ {
				if err := e.pushProfileRecord(ctx, resource, p.At(k), scope, defaultSession, eventsSession, stackTracesSession, stackFramesSession, executablesSession); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					if errors.Is(err, ErrInvalidTypeForBodyMapMode) {
						e.Logger.Warn("dropping profile record", zap.Error(err))
						continue
					}

					errs = append(errs, err)
				}
			}
		}
	}

	if err := defaultSession.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	if err := eventsSession.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	if err := stackTracesSession.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	if err := stackFramesSession.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	if err := executablesSession.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushProfileRecord(
	ctx context.Context,
	resource pcommon.Resource,
	record pprofile.Profile,
	scope pcommon.InstrumentationScope,
	defaultSession, eventsSession, stackTracesSession, stackFramesSession, executablesSession bulkIndexerSession,
) error {
	return e.model.encodeProfile(resource, scope, record, func(buf *bytes.Buffer, docID, index string) error {
		switch index {
		case otelserializer.StackTraceIndex:
			return stackTracesSession.Add(ctx, index, docID, buf, nil, docappender.ActionCreate)
		case otelserializer.StackFrameIndex:
			return stackFramesSession.Add(ctx, index, docID, buf, nil, docappender.ActionCreate)
		case otelserializer.AllEventsIndex:
			return eventsSession.Add(ctx, index, docID, buf, nil, docappender.ActionCreate)
		case otelserializer.ExecutablesIndex:
			return executablesSession.Add(ctx, index, docID, buf, nil, docappender.ActionUpdate)
		default:
			return defaultSession.Add(ctx, index, docID, buf, nil, docappender.ActionCreate)
		}
	})
}
