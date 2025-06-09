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
	"golang.org/x/sync/errgroup"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metricgroup"
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

	documentEncoders         [NumMappingModes]documentEncoder
	documentRouters          [NumMappingModes]documentRouter
	spanEventDocumentRouters [NumMappingModes]documentRouter
}

func newExporter(cfg *Config, set exporter.Settings, index string) (*elasticsearchExporter, error) {
	allowedMappingModes := cfg.allowedMappingModes()
	defaultMappingMode := allowedMappingModes[canonicalMappingModeName(cfg.Mapping.Mode)]
	exporter := &elasticsearchExporter{
		set:                 set,
		config:              cfg,
		index:               index,
		logstashFormat:      cfg.LogstashFormat,
		allowedMappingModes: allowedMappingModes,
		defaultMappingMode:  defaultMappingMode,
		bufferPool:          pool.NewBufferPool(),
	}
	for mappingMode := range NumMappingModes {
		encoder, err := newEncoder(mappingMode)
		if err != nil {
			return nil, err
		}
		exporter.documentEncoders[mappingMode] = encoder
		exporter.documentRouters[mappingMode] = newDocumentRouter(mappingMode, index, cfg)
		exporter.spanEventDocumentRouters[mappingMode] = newDocumentRouter(mappingMode, cfg.LogsIndex, cfg)
	}
	return exporter, nil
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

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	defaultMappingMode, err := e.getRequestMappingMode(ctx)
	if err != nil {
		return err
	}
	mappingModeSessions := mappingModeSessions{indexers: &e.bulkIndexers.modes}
	defer mappingModeSessions.End()

	var errs []error
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)
			scope := ill.Scope()
			mappingMode, err := e.getScopeMappingMode(scope, defaultMappingMode)
			if err != nil {
				return err
			}
			session := mappingModeSessions.StartSession(ctx, mappingMode)
			router := e.documentRouters[int(mappingMode)]
			encoder := e.documentEncoders[int(mappingMode)]

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

	if err := mappingModeSessions.Flush(ctx); err != nil {
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

func (e *elasticsearchExporter) pushMetricsData(ctx context.Context, metrics pmetric.Metrics) error {
	defaultMappingMode, err := e.getRequestMappingMode(ctx)
	if err != nil {
		return err
	}
	sessions := mappingModeSessions{indexers: &e.bulkIndexers.modes}
	defer sessions.End()

	type mappingIndexKey struct {
		mappingMode MappingMode
		index       elasticsearch.Index
	}

	// Maintain a 2 layer map to avoid storing lots of copies of index strings
	groupedDataPointsByIndex := make(map[mappingIndexKey]map[metricgroup.HashKey]*dataPointsGroup)

	var validationErrs []error // log instead of returning these so that upstream does not retry
	var errs []error
	for _, resourceMetrics := range metrics.ResourceMetrics().All() {
		resource := resourceMetrics.Resource()
		var hasher metricgroup.DataPointHasher
		var prevScopeMappingMode MappingMode
		for _, scopeMetrics := range resourceMetrics.ScopeMetrics().All() {
			scope := scopeMetrics.Scope()
			mappingMode, err := e.getScopeMappingMode(scope, defaultMappingMode)
			if err != nil {
				return err
			}
			router := e.documentRouters[int(mappingMode)]
			if hasher == nil || mappingMode != prevScopeMappingMode {
				hasher = newDataPointHasher(mappingMode)
				hasher.UpdateResource(resource)
			}
			prevScopeMappingMode = mappingMode

			hasher.UpdateScope(scope)
			for _, metric := range scopeMetrics.Metrics().All() {
				upsertDataPoint := func(dp datapoints.DataPoint) error {
					index, err := router.routeDataPoint(resource, scope, dp.Attributes())
					if err != nil {
						return err
					}
					key := mappingIndexKey{
						mappingMode: mappingMode,
						index:       index,
					}
					groupedDataPoints, ok := groupedDataPointsByIndex[key]
					if !ok {
						groupedDataPoints = make(map[metricgroup.HashKey]*dataPointsGroup)
						groupedDataPointsByIndex[key] = groupedDataPoints
					}
					hasher.UpdateDataPoint(dp)
					hashKey := hasher.HashKey()

					if dpGroup, ok := groupedDataPoints[hashKey]; !ok {
						groupedDataPoints[hashKey] = &dataPointsGroup{
							resource:          resource,
							resourceSchemaURL: resourceMetrics.SchemaUrl(),
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

	for key, groupedDataPoints := range groupedDataPointsByIndex {
		for _, dpGroup := range groupedDataPoints {
			buf := e.bufferPool.NewPooledBuffer()
			encoder := e.documentEncoders[int(key.mappingMode)]
			session := sessions.StartSession(ctx, key.mappingMode)

			dynamicTemplates, err := encoder.encodeMetrics(
				encodingContext{
					resource:          dpGroup.resource,
					resourceSchemaURL: dpGroup.resourceSchemaURL,
					scope:             dpGroup.scope,
					scopeSchemaURL:    dpGroup.scopeSchemaURL,
				},
				dpGroup.dataPoints,
				&validationErrs,
				key.index,
				buf.Buffer,
			)
			if err != nil {
				buf.Recycle()
				errs = append(errs, err)
				continue
			}
			if err := session.Add(ctx, key.index.Index, "", "", buf, dynamicTemplates, docappender.ActionCreate); err != nil {
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

	if err := sessions.Flush(ctx); err != nil {
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
	defaultMappingMode, err := e.getRequestMappingMode(ctx)
	if err != nil {
		return err
	}
	sessions := mappingModeSessions{indexers: &e.bulkIndexers.modes}
	defer sessions.End()

	var errs []error
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			scope := scopeSpan.Scope()
			mappingMode, err := e.getScopeMappingMode(scope, defaultMappingMode)
			if err != nil {
				return err
			}
			session := sessions.StartSession(ctx, mappingMode)
			router := e.documentRouters[int(mappingMode)]
			spanEventRouter := e.spanEventDocumentRouters[int(mappingMode)]
			encoder := e.documentEncoders[int(mappingMode)]

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

	if err := sessions.Flush(ctx); err != nil {
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
	defaultMappingMode, err := e.getRequestMappingMode(ctx)
	if err != nil {
		return err
	}

	var sessions sessionList
	defer sessions.End()
	startSession := func(indexer bulkIndexer) bulkIndexerSession {
		session := indexer.StartSession(ctx)
		sessions = append(sessions, session)
		return session
	}
	eventsSession := startSession(e.bulkIndexers.profilingEvents)
	stackTracesSession := startSession(e.bulkIndexers.profilingStackTraces)
	stackFramesSession := startSession(e.bulkIndexers.profilingStackFrames)
	executablesSession := startSession(e.bulkIndexers.profilingExecutables)

	// scopeMappingModeSessions is used to create the default session according to
	// the specified mapping mode.
	scopeMappingModeSessions := mappingModeSessions{indexers: &e.bulkIndexers.modes}
	defer scopeMappingModeSessions.End()
	dic := pd.ProfilesDictionary()

	var errs []error
	for _, rp := range pd.ResourceProfiles().All() {
		resource := rp.Resource()
		for _, sp := range rp.ScopeProfiles().All() {
			scope := sp.Scope()
			mappingMode, err := e.getScopeMappingMode(scope, defaultMappingMode)
			if err != nil {
				return err
			}
			defaultSession := scopeMappingModeSessions.StartSession(ctx, mappingMode)
			encoder := e.documentEncoders[int(mappingMode)]

			for _, profile := range sp.Profiles().All() {
				ec := encodingContext{
					resource:          resource,
					resourceSchemaURL: rp.SchemaUrl(),
					scope:             scope,
					scopeSchemaURL:    sp.SchemaUrl(),
				}
				if err := e.pushProfileRecord(
					ctx, encoder, ec, dic, profile, defaultSession, eventsSession,
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

	if err := sessions.Flush(ctx); err != nil {
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
	dic pprofile.ProfilesDictionary,
	profile pprofile.Profile,
	defaultSession, eventsSession, stackTracesSession, stackFramesSession, executablesSession bulkIndexerSession,
) error {
	return encoder.encodeProfile(ec, dic, profile, func(buf *bytes.Buffer, docID, index string) error {
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

// mappingModeSessions holds mapping-mode specific bulk indexer sessions.
type mappingModeSessions struct {
	indexers *[NumMappingModes]bulkIndexer
	sessions [NumMappingModes]bulkIndexerSession
	sessionList
}

// StartSession starts a new session for the given mapping mode if one has
// not yet been started, otherwise it returns the existing session.
//
// Note: this is not safe for concurrent use. It is expected to be used
// within a single Consume* call.
func (s *mappingModeSessions) StartSession(ctx context.Context, mappingMode MappingMode) bulkIndexerSession {
	if session := s.sessions[int(mappingMode)]; session != nil {
		return session
	}
	session := s.indexers[int(mappingMode)].StartSession(ctx)
	s.sessions[mappingMode] = session
	s.sessionList = append(s.sessionList, session)
	return session
}

// sessionList holds a list of bulkIndexerSession instances.
//
// This provides Flush and End methods that flush/end all sessions in the list.
type sessionList []bulkIndexerSession

// Flush concurrently flushes all sessions.
func (sessions *sessionList) Flush(ctx context.Context) error {
	var g errgroup.Group
	for _, session := range *sessions {
		g.Go(func() error {
			return session.Flush(ctx)
		})
	}
	return g.Wait()
}

// End ends all sessions.
func (sessions *sessionList) End() {
	for _, session := range *sessions {
		session.End()
	}
}

func (e *elasticsearchExporter) getRequestMappingMode(ctx context.Context) (MappingMode, error) {
	const metadataKey = "x-elastic-mapping-mode"

	values := client.FromContext(ctx).Metadata.Get(metadataKey)
	switch n := len(values); n {
	case 0:
		return e.defaultMappingMode, nil
	case 1:
		mode, err := e.parseMappingMode(values[0])
		if err != nil {
			return -1, fmt.Errorf("invalid context mapping mode: %w", err)
		}
		return mode, nil
	default:
		return -1, fmt.Errorf("expected one value for client metadata key %q, got %d", metadataKey, n)
	}
}

func (e *elasticsearchExporter) getScopeMappingMode(
	scope pcommon.InstrumentationScope, defaultMode MappingMode,
) (MappingMode, error) {
	attr, ok := scope.Attributes().Get(elasticsearch.MappingModeAttributeName)
	if !ok {
		return defaultMode, nil
	}
	mode, err := e.parseMappingMode(attr.AsString())
	if err != nil {
		return -1, fmt.Errorf("invalid scope mapping mode: %w", err)
	}
	return mode, nil
}

func (e *elasticsearchExporter) parseMappingMode(s string) (MappingMode, error) {
	mode, ok := e.allowedMappingModes[canonicalMappingModeName(s)]
	if !ok {
		return -1, fmt.Errorf(
			"unsupported mapping mode %q, expected one of %q",
			s, e.config.Mapping.AllowedModes,
		)
	}
	return mode, nil
}

func newDataPointHasher(mode MappingMode) metricgroup.DataPointHasher {
	switch mode {
	case MappingOTel:
		return &metricgroup.OTelDataPointHasher{}
	default:
		// Defaults to ECS for backward compatibility
		return &metricgroup.ECSDataPointHasher{}
	}
}
