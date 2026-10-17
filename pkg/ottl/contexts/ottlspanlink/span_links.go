// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlspanlink // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanlink"

import (
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspanlink"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/logging"
)

var tcPool = sync.Pool{
	New: func() any {
		return &TransformContext{cache: pcommon.NewMap()}
	},
}

const ContextName = ctxspanlink.Name

var _ zapcore.ObjectMarshaler = (*TransformContext)(nil)

type TransformContext struct {
	resourceSpans ptrace.ResourceSpans
	scopeSpans    ptrace.ScopeSpans
	span          ptrace.Span
	spanLink      ptrace.SpanLink
	cache         pcommon.Map
	externalCache *pcommon.Map
}

// MarshalLogObject serializes the TransformContext into a zapcore.ObjectEncoder for logging.
func (tCtx *TransformContext) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	err := encoder.AddObject("resource", logging.Resource(tCtx.GetResource()))
	err = errors.Join(err, encoder.AddObject("scope", logging.InstrumentationScope(tCtx.GetInstrumentationScope())))
	err = errors.Join(err, encoder.AddObject("span", logging.Span(tCtx.span)))
	err = errors.Join(err, encoder.AddObject("spanlink", logging.SpanLink(tCtx.spanLink)))
	err = errors.Join(err, encoder.AddObject("cache", logging.Map(getCache(tCtx))))
	return err
}

type TransformContextOption func(*TransformContext)

// WithCache sets an external shared cache on the TransformContext.
// When set, the cache is shared across multiple TransformContext instances.
func WithCache(cache *pcommon.Map) TransformContextOption {
	return func(tCtx *TransformContext) {
		if cache != nil {
			tCtx.externalCache = cache
		}
	}
}

func NewTransformContext(resourceSpans ptrace.ResourceSpans, scopeSpans ptrace.ScopeSpans, span ptrace.Span, spanLink ptrace.SpanLink, options ...TransformContextOption) *TransformContext {
	tCtx := tcPool.Get().(*TransformContext)
	tCtx.resourceSpans = resourceSpans
	tCtx.scopeSpans = scopeSpans
	tCtx.span = span
	tCtx.spanLink = spanLink
	for _, opt := range options {
		opt(tCtx)
	}
	return tCtx
}

func (tCtx *TransformContext) Close() {
	tCtx.resourceSpans = ptrace.ResourceSpans{}
	tCtx.scopeSpans = ptrace.ScopeSpans{}
	tCtx.span = ptrace.Span{}
	tCtx.spanLink = ptrace.SpanLink{}
	tCtx.cache.Clear()
	tCtx.externalCache = nil
	tcPool.Put(tCtx)
}

func (tCtx *TransformContext) GetSpanLink() ptrace.SpanLink {
	return tCtx.spanLink
}

func (tCtx *TransformContext) GetSpan() ptrace.Span {
	return tCtx.span
}

func (tCtx *TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return tCtx.scopeSpans.Scope()
}

func (tCtx *TransformContext) GetResource() pcommon.Resource {
	return tCtx.resourceSpans.Resource()
}

func (tCtx *TransformContext) GetScopeSchemaURLItem() ottl.SchemaURLItem {
	return tCtx.scopeSpans
}

func (tCtx *TransformContext) GetResourceSchemaURLItem() ottl.SchemaURLItem {
	return tCtx.resourceSpans
}

// EnablePathContextNames enables the support for path's context names on statements.
// When this option is configured, all statement's paths must have a valid context prefix.
func EnablePathContextNames() ottl.Option[*TransformContext] {
	return func(p *ottl.Parser[*TransformContext]) {
		ottl.WithPathContextNames[*TransformContext]([]string{
			ctxspanlink.Name,
			ctxspan.Name,
			ctxresource.Name,
			ctxscope.LegacyName,
			ctxscope.Name,
			ctxotelcol.Name,
		})(p)
	}
}

type StatementSequenceOption func(*ottl.StatementSequence[*TransformContext])

func WithStatementSequenceErrorMode(errorMode ottl.ErrorMode) StatementSequenceOption {
	return func(s *ottl.StatementSequence[*TransformContext]) {
		ottl.WithStatementSequenceErrorMode[*TransformContext](errorMode)(s)
	}
}

func NewStatementSequence(statements []*ottl.Statement[*TransformContext], telemetrySettings component.TelemetrySettings, options ...StatementSequenceOption) ottl.StatementSequence[*TransformContext] {
	s := ottl.NewStatementSequence(statements, telemetrySettings)
	for _, op := range options {
		op(&s)
	}
	return s
}

type ConditionSequenceOption func(*ottl.ConditionSequence[*TransformContext])

func WithConditionSequenceErrorMode(errorMode ottl.ErrorMode) ConditionSequenceOption {
	return func(c *ottl.ConditionSequence[*TransformContext]) {
		ottl.WithConditionSequenceErrorMode[*TransformContext](errorMode)(c)
	}
}

func NewConditionSequence(conditions []*ottl.Condition[*TransformContext], telemetrySettings component.TelemetrySettings, options ...ConditionSequenceOption) ottl.ConditionSequence[*TransformContext] {
	c := ottl.NewConditionSequence(conditions, telemetrySettings)
	for _, op := range options {
		op(&c)
	}
	return c
}

func NewParser(
	functions map[string]ottl.Factory[*TransformContext],
	telemetrySettings component.TelemetrySettings,
	options ...ottl.Option[*TransformContext],
) (ottl.Parser[*TransformContext], error) {
	return ctxcommon.NewParser(
		functions,
		telemetrySettings,
		pathExpressionParser(getCache),
		parseEnum,
		options...,
	)
}

func parseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	if val != nil {
		if enum, ok := ctxspan.SymbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, errors.New("enum symbol not provided")
}

func getCache(tCtx *TransformContext) pcommon.Map {
	if tCtx.externalCache != nil {
		return *tCtx.externalCache
	}
	return tCtx.cache
}

func pathExpressionParser(cacheGetter ctxcache.Getter[*TransformContext]) ottl.PathExpressionParser[*TransformContext] {
	return ctxcommon.PathExpressionParser(
		ctxspanlink.Name,
		ctxspanlink.DocRef,
		cacheGetter,
		map[string]ottl.PathExpressionParser[*TransformContext]{
			ctxresource.Name:    ctxresource.PathGetSetter[*TransformContext],
			ctxscope.Name:       ctxscope.PathGetSetter[*TransformContext],
			ctxscope.LegacyName: ctxscope.PathGetSetter[*TransformContext],
			ctxspan.Name:        ctxspan.PathGetSetter[*TransformContext],
			ctxspanlink.Name:    ctxspanlink.PathGetSetter[*TransformContext],
			ctxotelcol.Name:     ctxotelcol.PathGetSetter[*TransformContext],
		},
	)
}
