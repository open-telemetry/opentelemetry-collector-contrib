// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottllog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/logging"
)

// Experimental: *NOTE* this constant is subject to change or removal in the future.
const ContextName = ctxlog.Name

var (
	_ internal.ResourceContext             = (*TransformContext)(nil)
	_ internal.InstrumentationScopeContext = (*TransformContext)(nil)
	_ zapcore.ObjectMarshaler              = (*TransformContext)(nil)
)

type TransformContext struct {
	logRecord            plog.LogRecord
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
	cache                pcommon.Map
	scopeLogs            plog.ScopeLogs
	resourceLogs         plog.ResourceLogs
}

type logRecord plog.LogRecord

func (l logRecord) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	lr := plog.LogRecord(l)
	spanID := lr.SpanID()
	traceID := lr.TraceID()
	err := encoder.AddObject("attributes", logging.Map(lr.Attributes()))
	encoder.AddString("body", lr.Body().AsString())
	encoder.AddUint32("dropped_attribute_count", lr.DroppedAttributesCount())
	encoder.AddUint32("flags", uint32(lr.Flags()))
	encoder.AddUint64("observed_time_unix_nano", uint64(lr.ObservedTimestamp()))
	encoder.AddInt32("severity_number", int32(lr.SeverityNumber()))
	encoder.AddString("severity_text", lr.SeverityText())
	encoder.AddString("span_id", hex.EncodeToString(spanID[:]))
	encoder.AddUint64("time_unix_nano", uint64(lr.Timestamp()))
	encoder.AddString("trace_id", hex.EncodeToString(traceID[:]))
	return err
}

func (tCtx TransformContext) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	err := encoder.AddObject("resource", logging.Resource(tCtx.resource))
	err = errors.Join(err, encoder.AddObject("scope", logging.InstrumentationScope(tCtx.instrumentationScope)))
	err = errors.Join(err, encoder.AddObject("log_record", logRecord(tCtx.logRecord)))
	err = errors.Join(err, encoder.AddObject("cache", logging.Map(tCtx.cache)))
	return err
}

type Option func(*ottl.Parser[TransformContext])

type TransformContextOption func(*TransformContext)

func NewTransformContext(logRecord plog.LogRecord, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource, scopeLogs plog.ScopeLogs, resourceLogs plog.ResourceLogs, options ...TransformContextOption) TransformContext {
	tc := TransformContext{
		logRecord:            logRecord,
		instrumentationScope: instrumentationScope,
		resource:             resource,
		cache:                pcommon.NewMap(),
		scopeLogs:            scopeLogs,
		resourceLogs:         resourceLogs,
	}
	for _, opt := range options {
		opt(&tc)
	}
	return tc
}

// Experimental: *NOTE* this option is subject to change or removal in the future.
func WithCache(cache *pcommon.Map) TransformContextOption {
	return func(p *TransformContext) {
		if cache != nil {
			p.cache = *cache
		}
	}
}

func (tCtx TransformContext) GetLogRecord() plog.LogRecord {
	return tCtx.logRecord
}

func (tCtx TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return tCtx.instrumentationScope
}

func (tCtx TransformContext) GetResource() pcommon.Resource {
	return tCtx.resource
}

func (tCtx TransformContext) getCache() pcommon.Map {
	return tCtx.cache
}

func (tCtx TransformContext) GetScopeSchemaURLItem() internal.SchemaURLItem {
	return tCtx.scopeLogs
}

func (tCtx TransformContext) GetResourceSchemaURLItem() internal.SchemaURLItem {
	return tCtx.resourceLogs
}

func NewParser(functions map[string]ottl.Factory[TransformContext], telemetrySettings component.TelemetrySettings, options ...Option) (ottl.Parser[TransformContext], error) {
	pep := pathExpressionParser{telemetrySettings}
	p, err := ottl.NewParser[TransformContext](
		functions,
		pep.parsePath,
		telemetrySettings,
		ottl.WithEnumParser[TransformContext](parseEnum),
	)
	if err != nil {
		return ottl.Parser[TransformContext]{}, err
	}
	for _, opt := range options {
		opt(&p)
	}
	return p, nil
}

// EnablePathContextNames enables the support to path's context names on statements.
// When this option is configured, all statement's paths must have a valid context prefix,
// otherwise an error is reported.
//
// Experimental: *NOTE* this option is subject to change or removal in the future.
func EnablePathContextNames() Option {
	return func(p *ottl.Parser[TransformContext]) {
		ottl.WithPathContextNames[TransformContext]([]string{
			ctxlog.Name,
			ctxscope.LegacyName,
			ctxresource.Name,
		})(p)
	}
}

type StatementSequenceOption func(*ottl.StatementSequence[TransformContext])

func WithStatementSequenceErrorMode(errorMode ottl.ErrorMode) StatementSequenceOption {
	return func(s *ottl.StatementSequence[TransformContext]) {
		ottl.WithStatementSequenceErrorMode[TransformContext](errorMode)(s)
	}
}

func NewStatementSequence(statements []*ottl.Statement[TransformContext], telemetrySettings component.TelemetrySettings, options ...StatementSequenceOption) ottl.StatementSequence[TransformContext] {
	s := ottl.NewStatementSequence(statements, telemetrySettings)
	for _, op := range options {
		op(&s)
	}
	return s
}

type ConditionSequenceOption func(*ottl.ConditionSequence[TransformContext])

func WithConditionSequenceErrorMode(errorMode ottl.ErrorMode) ConditionSequenceOption {
	return func(c *ottl.ConditionSequence[TransformContext]) {
		ottl.WithConditionSequenceErrorMode[TransformContext](errorMode)(c)
	}
}

func NewConditionSequence(conditions []*ottl.Condition[TransformContext], telemetrySettings component.TelemetrySettings, options ...ConditionSequenceOption) ottl.ConditionSequence[TransformContext] {
	c := ottl.NewConditionSequence(conditions, telemetrySettings)
	for _, op := range options {
		op(&c)
	}
	return c
}

func parseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	if val != nil {
		if enum, ok := ctxlog.SymbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, fmt.Errorf("enum symbol not provided")
}

type pathExpressionParser struct {
	telemetrySettings component.TelemetrySettings
}

func (pep *pathExpressionParser) parsePath(path ottl.Path[TransformContext]) (ottl.GetSetter[TransformContext], error) {
	if path == nil {
		return nil, ctxerror.New("nil", "nil", ctxlog.Name, ctxlog.DocRef)
	}
	// Higher contexts parsing
	if path.Context() != "" && path.Context() != ctxlog.Name {
		return pep.parseHigherContextPath(path.Context(), path)
	}
	// Backward compatibility with paths without context
	if path.Context() == "" && (path.Name() == ctxresource.Name || path.Name() == ctxscope.LegacyName) {
		return pep.parseHigherContextPath(path.Name(), path.Next())
	}

	switch path.Name() {
	case "cache":
		if path.Keys() == nil {
			return accessCache(), nil
		}
		return accessCacheKey(path.Keys()), nil
	default:
		return ctxlog.PathGetSetter[TransformContext](path)
	}
}

func (pep *pathExpressionParser) parseHigherContextPath(context string, path ottl.Path[TransformContext]) (ottl.GetSetter[TransformContext], error) {
	switch context {
	case ctxresource.Name:
		return internal.ResourcePathGetSetter(ctxlog.Name, path)
	case ctxscope.LegacyName:
		return internal.ScopePathGetSetter(ctxlog.Name, path)
	default:
		var fullPath string
		if path != nil {
			fullPath = path.String()
		}
		return nil, ctxerror.New(context, fullPath, ctxlog.Name, ctxlog.DocRef)
	}
}

func accessCache() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.getCache(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if m, ok := val.(pcommon.Map); ok {
				m.CopyTo(tCtx.getCache())
			}
			return nil
		},
	}
}

func accessCacheKey(key []ottl.Key[TransformContext]) ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx context.Context, tCtx TransformContext) (any, error) {
			return ctxutil.GetMapValue[TransformContext](ctx, tCtx, tCtx.getCache(), key)
		},
		Setter: func(ctx context.Context, tCtx TransformContext, val any) error {
			return ctxutil.SetMapValue[TransformContext](ctx, tCtx, tCtx.getCache(), key, val)
		},
	}
}
