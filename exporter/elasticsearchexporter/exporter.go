// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/elastic/go-docappender/v2"
	"go.opentelemetry.io/collector/client"
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
	set                 exporter.Settings
	config              *Config
	index               string
	logstashFormat      LogstashFormatSettings
	defaultMappingMode  MappingMode
	allowedMappingModes map[string]MappingMode
	bulkIndexers        bulkIndexers
	bufferPool          *pool.BufferPool
	encoders            map[MappingMode]documentEncoder
}

func newExporter(cfg *Config, set exporter.Settings, index string) (*elasticsearchExporter, error) {
	allowedMappingModes := cfg.allowedMappingModes()
	defaultMappingMode := allowedMappingModes[canonicalMappingModeName(cfg.Mapping.Mode)]

	encoders := map[MappingMode]documentEncoder{}
	for i := range NumMappingModes {
		enc, err := newEncoder(i)
		if err != nil {
			return nil, err
		}
		encoders[i] = enc
	}

	return &elasticsearchExporter{
		set:                 set,
		config:              cfg,
		index:               index,
		logstashFormat:      cfg.LogstashFormat,
		allowedMappingModes: allowedMappingModes,
		defaultMappingMode:  defaultMappingMode,
		bufferPool:          pool.NewBufferPool(),
		encoders:            encoders,
	}, nil
}

func (e *elasticsearchExporter) Start(ctx context.Context, host component.Host) error {
	if err := e.bulkIndexers.start(ctx, e.config, e.set, host, e.allowedMappingModes); err != nil {
		return fmt.Errorf("error starting bulk indexers: %w", err)
	}
	return nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	if err := e.bulkIndexers.shutdown(ctx); err != nil {
		return fmt.Errorf("error shutting down bulk indexers: %w", err)
	}
	return nil
}

func (e *elasticsearchExporter) getEncoder(m MappingMode) (documentEncoder, error) {
	if enc, ok := e.encoders[m]; ok {
		return enc, nil
	}

	return nil, fmt.Errorf("no encoder setup for mapping mode %s", m)
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	mappingMode, err := e.getMappingMode(ctx)
	if err != nil {
		return err
	}
	router := newDocumentRouter(mappingMode, e.index, e.config)
	encoder, err := e.getEncoder(mappingMode)
	if err != nil {
		return err
	}

	session, err := e.bulkIndexers.modes[mappingMode].StartSession(ctx)
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
			ec := encodingContext{
				resource:          resource,
				resourceSchemaURL: rl.SchemaUrl(),
				scope:             scope,
				scopeSchemaURL:    ill.SchemaUrl(),
			}

			logs := ill.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				if err := e.pushLogRecord(ctx, router, encoder, ec, logs.At(k), session); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					if errors.Is(err, ErrInvalidTypeForBodyMapMode) {
						e.set.Logger.Warn("dropping log record", zap.Error(err))
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
	encoder documentEncoder,
	ec encodingContext,
	record plog.LogRecord,
	bulkIndexerSession bulkIndexerSession,
) error {
	index, err := router.routeLogRecord(ec.resource, ec.scope, record.Attributes())
	if err != nil {
		return err
	}

	buf := e.bufferPool.NewPooledBuffer()
	docID := e.extractDocumentIDAttribute(record.Attributes())
	pipeline := e.extractDocumentPipelineAttribute(record.Attributes())
	if err := encoder.encodeLog(ec, record, index, buf.Buffer); err != nil {
		buf.Recycle()
		return fmt.Errorf("failed to encode log event: %w", err)
	}

	// not recycling after Add returns an error as we don't know if it's already recycled
	return bulkIndexerSession.Add(ctx, index.Index, docID, pipeline, buf, nil, docappender.ActionCreate)
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
	mappingMode, err := e.getMappingMode(ctx)
	if err != nil {
		return err
	}
	router := newDocumentRouter(mappingMode, e.index, e.config)
	hasher := newDataPointHasher(mappingMode)
	encoder, err := e.getEncoder(mappingMode)
	if err != nil {
		return err
	}

	session, err := e.bulkIndexers.modes[mappingMode].StartSession(ctx)
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
							resource:          resource,
							resourceSchemaURL: resourceMetric.SchemaUrl(),
							scope:             scope,
							scopeSchemaURL:    scopeMetrics.SchemaUrl(),
							dataPoints:        []datapoints.DataPoint{dp},
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
			dynamicTemplates, err := encoder.encodeMetrics(
				encodingContext{
					resource:          dpGroup.resource,
					resourceSchemaURL: dpGroup.resourceSchemaURL,
					scope:             dpGroup.scope,
					scopeSchemaURL:    dpGroup.scopeSchemaURL,
				},
				dpGroup.dataPoints,
				&validationErrs,
				index,
				buf.Buffer,
			)
			if err != nil {
				buf.Recycle()
				errs = append(errs, err)
				continue
			}
			if err := session.Add(ctx, index.Index, "", "", buf, dynamicTemplates, docappender.ActionCreate); err != nil {
				// not recycling after Add returns an error as we don't know if it's already recycled
				if cerr := ctx.Err(); cerr != nil {
					return cerr
				}
				errs = append(errs, err)
			}
		}
	}
	if len(validationErrs) > 0 {
		e.set.Logger.Warn("validation errors", zap.Error(errors.Join(validationErrs...)))
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
	mappingMode, err := e.getMappingMode(ctx)
	if err != nil {
		return err
	}
	router := newDocumentRouter(mappingMode, e.index, e.config)
	spanEventRouter := newDocumentRouter(mappingMode, e.config.LogsIndex, e.config)
	encoder, err := e.getEncoder(mappingMode)
	if err != nil {
		return err
	}

	session, err := e.bulkIndexers.modes[mappingMode].StartSession(ctx)
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
			ec := encodingContext{
				resource:          resource,
				resourceSchemaURL: il.SchemaUrl(),
				scope:             scope,
				scopeSchemaURL:    scopeSpan.SchemaUrl(),
			}

			spans := scopeSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if err := e.pushTraceRecord(ctx, router, encoder, ec, span, session); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
				for ii := 0; ii < span.Events().Len(); ii++ {
					spanEvent := span.Events().At(ii)
					if err := e.pushSpanEvent(ctx, spanEventRouter, encoder, ec, span, spanEvent, session); err != nil {
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
	encoder documentEncoder,
	ec encodingContext,
	span ptrace.Span,
	bulkIndexerSession bulkIndexerSession,
) error {
	index, err := router.routeSpan(ec.resource, ec.scope, span.Attributes())
	if err != nil {
		return err
	}

	buf := e.bufferPool.NewPooledBuffer()
	if err := encoder.encodeSpan(ec, span, index, buf.Buffer); err != nil {
		buf.Recycle()
		return fmt.Errorf("failed to encode trace record: %w", err)
	}
	// not recycling after Add returns an error as we don't know if it's already recycled
	return bulkIndexerSession.Add(ctx, index.Index, "", "", buf, nil, docappender.ActionCreate)
}

func (e *elasticsearchExporter) pushSpanEvent(
	ctx context.Context,
	router documentRouter,
	encoder documentEncoder,
	ec encodingContext,
	span ptrace.Span,
	spanEvent ptrace.SpanEvent,
	bulkIndexerSession bulkIndexerSession,
) error {
	index, err := router.routeSpanEvent(ec.resource, ec.scope, spanEvent.Attributes())
	if err != nil {
		return err
	}

	buf := e.bufferPool.NewPooledBuffer()
	if err := encoder.encodeSpanEvent(ec, span, spanEvent, index, buf.Buffer); err != nil || buf.Buffer.Len() == 0 {
		buf.Recycle()
		return err
	}
	// not recycling after Add returns an error as we don't know if it's already recycled
	return bulkIndexerSession.Add(ctx, index.Index, "", "", buf, nil, docappender.ActionCreate)
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

func (e *elasticsearchExporter) extractDocumentPipelineAttribute(m pcommon.Map) string {
	if !e.config.LogsDynamicPipeline.Enabled {
		return ""
	}

	v, ok := m.Get(elasticsearch.DocumentPipelineAttributeName)
	if !ok {
		return ""
	}
	return v.AsString()
}

func (e *elasticsearchExporter) pushProfilesData(ctx context.Context, pd pprofile.Profiles) error {
	// TODO add support for routing profiles to different data_stream.namespaces?
	mappingMode, err := e.getMappingMode(ctx)
	if err != nil {
		return err
	}
	encoder, err := e.getEncoder(mappingMode)
	if err != nil {
		return err
	}

	defaultSession, err := e.bulkIndexers.modes[mappingMode].StartSession(ctx)
	if err != nil {
		return err
	}
	defer defaultSession.End()
	eventsSession, err := e.bulkIndexers.profilingEvents.StartSession(ctx)
	if err != nil {
		return err
	}
	defer eventsSession.End()
	stackTracesSession, err := e.bulkIndexers.profilingStackTraces.StartSession(ctx)
	if err != nil {
		return err
	}
	defer stackTracesSession.End()
	stackFramesSession, err := e.bulkIndexers.profilingStackFrames.StartSession(ctx)
	if err != nil {
		return err
	}
	defer stackFramesSession.End()
	executablesSession, err := e.bulkIndexers.profilingExecutables.StartSession(ctx)
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
				ec := encodingContext{
					resource:          resource,
					resourceSchemaURL: rp.SchemaUrl(),
					scope:             scope,
					scopeSchemaURL:    sp.SchemaUrl(),
				}
				if err := e.pushProfileRecord(
					ctx, encoder, ec, p.At(k), defaultSession, eventsSession,
					stackTracesSession, stackFramesSession, executablesSession,
				); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					if errors.Is(err, ErrInvalidTypeForBodyMapMode) {
						e.set.Logger.Warn("dropping profile record", zap.Error(err))
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
	encoder documentEncoder,
	ec encodingContext,
	profile pprofile.Profile,
	defaultSession, eventsSession, stackTracesSession, stackFramesSession, executablesSession bulkIndexerSession,
) error {
	return encoder.encodeProfile(ec, profile, func(buf *bytes.Buffer, docID, index string) error {
		switch index {
		case otelserializer.StackTraceIndex:
			return stackTracesSession.Add(ctx, index, docID, "", buf, nil, docappender.ActionCreate)
		case otelserializer.StackFrameIndex:
			return stackFramesSession.Add(ctx, index, docID, "", buf, nil, docappender.ActionCreate)
		case otelserializer.AllEventsIndex:
			return eventsSession.Add(ctx, index, docID, "", buf, nil, docappender.ActionCreate)
		case otelserializer.ExecutablesIndex:
			return executablesSession.Add(ctx, index, docID, "", buf, nil, docappender.ActionUpdate)
		case otelserializer.ExecutablesSymQueueIndex, otelserializer.LeafFramesSymQueueIndex:
			// These regular indices have a low write-frequency and can share the executablesSession.
			return executablesSession.Add(ctx, index, docID, "", buf, nil, docappender.ActionCreate)
		default:
			return defaultSession.Add(ctx, index, docID, "", buf, nil, docappender.ActionCreate)
		}
	})
}

func (e *elasticsearchExporter) getMappingMode(ctx context.Context) (MappingMode, error) {
	const metadataKey = "x-elastic-mapping-mode"

	values := client.FromContext(ctx).Metadata.Get(metadataKey)
	switch n := len(values); n {
	case 0:
		return e.defaultMappingMode, nil
	case 1:
		name := values[0]
		mode, ok := e.allowedMappingModes[canonicalMappingModeName(name)]
		if !ok {
			return -1, fmt.Errorf(
				"unsupported mapping mode %q, expected one of %q",
				name, e.config.Mapping.AllowedModes,
			)
		}
		return mode, nil
	default:
		return -1, fmt.Errorf("expected one value for %s, got %d", metadataKey, n)
	}
}
