// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlspanevent // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/logging"
)

// Experimental: *NOTE* this constant is subject to change or removal in the future.
const ContextName = ctxspanevent.Name

var (
	_ ctxresource.Context     = (*TransformContext)(nil)
	_ ctxscope.Context        = (*TransformContext)(nil)
	_ ctxspan.Context         = (*TransformContext)(nil)
	_ zapcore.ObjectMarshaler = (*TransformContext)(nil)
)

type TransformContext struct {
	spanEvent            ptrace.SpanEvent
	span                 ptrace.Span
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
	cache                pcommon.Map
	scopeSpans           ptrace.ScopeSpans
	resouceSpans         ptrace.ResourceSpans
	eventIndex           *int64
}

func (tCtx TransformContext) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	err := encoder.AddObject("resource", logging.Resource(tCtx.resource))
	err = errors.Join(err, encoder.AddObject("scope", logging.InstrumentationScope(tCtx.instrumentationScope)))
	err = errors.Join(err, encoder.AddObject("span", logging.Span(tCtx.span)))
	err = errors.Join(err, encoder.AddObject("spanevent", logging.SpanEvent(tCtx.spanEvent)))
	err = errors.Join(err, encoder.AddObject("cache", logging.Map(tCtx.cache)))
	if tCtx.eventIndex != nil {
		encoder.AddInt64("event_index", *tCtx.eventIndex)
	}
	return err
}

type TransformContextOption func(*TransformContext)

func NewTransformContext(spanEvent ptrace.SpanEvent, span ptrace.Span, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource, scopeSpans ptrace.ScopeSpans, resourceSpans ptrace.ResourceSpans, options ...TransformContextOption) TransformContext {
	tc := TransformContext{
		spanEvent:            spanEvent,
		span:                 span,
		instrumentationScope: instrumentationScope,
		resource:             resource,
		cache:                pcommon.NewMap(),
		scopeSpans:           scopeSpans,
		resouceSpans:         resourceSpans,
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

// WithEventIndex sets the index of the SpanEvent within the span, to make it accessible via the event_index property of its context.
// The index must be greater than or equal to zero, otherwise the given val will not be applied.
func WithEventIndex(eventIndex int64) TransformContextOption {
	return func(p *TransformContext) {
		p.eventIndex = &eventIndex
	}
}

func (tCtx TransformContext) GetSpanEvent() ptrace.SpanEvent {
	return tCtx.spanEvent
}

func (tCtx TransformContext) GetSpan() ptrace.Span {
	return tCtx.span
}

func (tCtx TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return tCtx.instrumentationScope
}

func (tCtx TransformContext) GetResource() pcommon.Resource {
	return tCtx.resource
}

func (tCtx TransformContext) GetScopeSchemaURLItem() ctxcommon.SchemaURLItem {
	return tCtx.scopeSpans
}

func (tCtx TransformContext) GetResourceSchemaURLItem() ctxcommon.SchemaURLItem {
	return tCtx.resouceSpans
}

func (tCtx TransformContext) GetEventIndex() (int64, error) {
	if tCtx.eventIndex != nil {
		if *tCtx.eventIndex < 0 {
			return 0, errors.New("found invalid value for 'event_index'")
		}
		return *tCtx.eventIndex, nil
	}
	return 0, errors.New("no 'event_index' property has been set")
}

func getCache(tCtx TransformContext) pcommon.Map {
	return tCtx.cache
}

type pathExpressionParser struct {
	telemetrySettings component.TelemetrySettings
	cacheGetSetter    ottl.PathExpressionParser[TransformContext]
}

func NewParser(functions map[string]ottl.Factory[TransformContext], telemetrySettings component.TelemetrySettings, options ...ottl.Option[TransformContext]) (ottl.Parser[TransformContext], error) {
	pep := pathExpressionParser{
		telemetrySettings: telemetrySettings,
		cacheGetSetter:    ctxcache.PathExpressionParser[TransformContext](getCache),
	}
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
func EnablePathContextNames() ottl.Option[TransformContext] {
	return func(p *ottl.Parser[TransformContext]) {
		ottl.WithPathContextNames[TransformContext]([]string{
			ctxspanevent.Name,
			ctxspan.Name,
			ctxresource.Name,
			ctxscope.LegacyName,
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
		if enum, ok := ctxspan.SymbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, fmt.Errorf("enum symbol not provided")
}

func (pep *pathExpressionParser) parsePath(path ottl.Path[TransformContext]) (ottl.GetSetter[TransformContext], error) {
	if path == nil {
		return nil, ctxerror.New("nil", "nil", ctxspanevent.Name, ctxspanevent.DocRef)
	}
	// Higher contexts parsing
	if path.Context() != "" && path.Context() != ctxspanevent.Name {
		return pep.parseHigherContextPath(path.Context(), path)
	}
	// Backward compatibility with paths without context
	if path.Context() == "" &&
		(path.Name() == ctxresource.Name ||
			path.Name() == ctxscope.LegacyName ||
			path.Name() == ctxspan.Name) {
		return pep.parseHigherContextPath(path.Name(), path.Next())
	}

	switch path.Name() {
	case "cache":
		return pep.cacheGetSetter(path)
	case "event_index":
		return accessSpanEventIndex(), nil
	default:
		return ctxspanevent.PathGetSetter(path)
	}
}

func (pep *pathExpressionParser) parseHigherContextPath(context string, path ottl.Path[TransformContext]) (ottl.GetSetter[TransformContext], error) {
	switch context {
	case ctxresource.Name:
		return ctxresource.PathGetSetter(ctxspanevent.Name, path)
	case ctxscope.LegacyName:
		return ctxscope.PathGetSetter(ctxspanevent.Name, path)
	case ctxspan.Name:
		return ctxspan.PathGetSetter(ctxspanevent.Name, path)
	default:
		var fullPath string
		if path != nil {
			fullPath = path.String()
		}
		return nil, ctxerror.New(context, fullPath, ctxspanevent.Name, ctxspanevent.DocRef)
	}
}

func accessSpanEventIndex() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetEventIndex()
		},
		Setter: func(_ context.Context, _ TransformContext, _ any) error {
			return errors.New("the 'event_index' path cannot be modified")
		},
	}
}
