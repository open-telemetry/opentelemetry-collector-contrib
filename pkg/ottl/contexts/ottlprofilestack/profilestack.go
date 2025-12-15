// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofilestack // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofilestack"

import (
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilesample"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilestack"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/logging"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/logprofile"
)

var tcPool = sync.Pool{
	New: func() any {
		return &TransformContext{cache: pcommon.NewMap()}
	},
}

// ContextName is the name of the context for profiles.
// Experimental: *NOTE* this constant is subject to change or removal in the future.
const ContextName = ctxprofilestack.Name

var (
	_ ctxresource.Context      = TransformContext{}
	_ ctxscope.Context         = TransformContext{}
	_ ctxprofilestack.Context  = TransformContext{}
	_ ctxprofilesample.Context = TransformContext{}
	_ ctxprofile.Context       = TransformContext{}
	_ zapcore.ObjectMarshaler  = TransformContext{}
)

// TransformContext represents a profile and its associated hierarchy.
type TransformContext struct {
	stack                pprofile.Stack
	sample               pprofile.Sample
	profile              pprofile.Profile
	dictionary           pprofile.ProfilesDictionary
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
	cache                pcommon.Map
	scopeProfiles        pprofile.ScopeProfiles
	resourceProfiles     pprofile.ResourceProfiles
}

// MarshalLogObject serializes the profile into a zapcore.ObjectEncoder for logging.
func (tCtx TransformContext) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	err := encoder.AddObject("resource", logging.Resource(tCtx.resource))
	err = errors.Join(err, encoder.AddObject("scope", logging.InstrumentationScope(tCtx.instrumentationScope)))
	err = errors.Join(err, encoder.AddObject("stack", logprofile.ProfileStack{Stack: tCtx.stack, Dictionary: tCtx.dictionary}))
	err = errors.Join(err, encoder.AddObject("cache", logging.Map(tCtx.cache)))
	return err
}

// TransformContextOption represents an option for configuring a TransformContext.
type TransformContextOption func(*TransformContext)

// NewTransformContext creates a new TransformContext with the provided parameters.
//
// Deprecated: Use NewTransformContextPtr.
func NewTransformContext(stack pprofile.Stack, sample pprofile.Sample, profile pprofile.Profile, dictionary pprofile.ProfilesDictionary, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource, scopeProfiles pprofile.ScopeProfiles, resourceProfiles pprofile.ResourceProfiles, options ...TransformContextOption) TransformContext {
	tc := TransformContext{
		stack:                stack,
		sample:               sample,
		profile:              profile,
		dictionary:           dictionary,
		instrumentationScope: instrumentationScope,
		resource:             resource,
		cache:                pcommon.NewMap(),
		scopeProfiles:        scopeProfiles,
		resourceProfiles:     resourceProfiles,
	}
	for _, opt := range options {
		opt(&tc)
	}
	return tc
}

// NewTransformContextPtr returns a new TransformContext with the provided parameters from a pool of contexts.
// Caller must call TransformContext.Close on the returned TransformContext.
func NewTransformContextPtr(resourceProfiles pprofile.ResourceProfiles, scopeProfiles pprofile.ScopeProfiles, stack pprofile.Stack, sample pprofile.Sample, profile pprofile.Profile, dictionary pprofile.ProfilesDictionary, options ...TransformContextOption) *TransformContext {
	tCtx := tcPool.Get().(*TransformContext)
	tCtx.stack = stack
	tCtx.sample = sample
	tCtx.profile = profile
	tCtx.dictionary = dictionary
	tCtx.instrumentationScope = scopeProfiles.Scope()
	tCtx.resource = resourceProfiles.Resource()
	tCtx.scopeProfiles = scopeProfiles
	tCtx.resourceProfiles = resourceProfiles
	for _, opt := range options {
		opt(tCtx)
	}
	return tCtx
}

// Close the current TransformContext.
// After this function returns this instance cannot be used.
func (tCtx *TransformContext) Close() {
	tCtx.stack = pprofile.NewStack()
	tCtx.sample = pprofile.NewSample()
	tCtx.profile = pprofile.NewProfile()
	tCtx.dictionary = pprofile.NewProfilesDictionary()
	tCtx.instrumentationScope = pcommon.InstrumentationScope{}
	tCtx.resource = pcommon.Resource{}
	tCtx.cache.Clear()
	tCtx.scopeProfiles = pprofile.ScopeProfiles{}
	tCtx.resourceProfiles = pprofile.ResourceProfiles{}
	tcPool.Put(tCtx)
}

// GetProfile returns the profile from the TransformContext.
func (tCtx TransformContext) GetProfile() pprofile.Profile {
	return tCtx.profile
}

// GetProfileSample returns the sample from the TransformContext.
func (tCtx TransformContext) GetProfileSample() pprofile.Sample {
	return tCtx.sample
}

// GetProfileStack returns the sample from the TransformContext.
func (tCtx TransformContext) GetProfileStack() pprofile.Stack {
	return tCtx.stack
}

// GetProfilesDictionary returns the profiles dictionary from the TransformContext.
func (tCtx TransformContext) GetProfilesDictionary() pprofile.ProfilesDictionary {
	return tCtx.dictionary
}

// GetInstrumentationScope returns the instrumentation scope from the TransformContext.
func (tCtx TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return tCtx.instrumentationScope
}

// GetResource returns the resource from the TransformContext.
func (tCtx TransformContext) GetResource() pcommon.Resource {
	return tCtx.resource
}

// GetScopeSchemaURLItem returns the scope schema URL item from the TransformContext.
func (tCtx TransformContext) GetScopeSchemaURLItem() ctxcommon.SchemaURLItem {
	return tCtx.scopeProfiles
}

// GetResourceSchemaURLItem returns the resource schema URL item from the TransformContext.
func (tCtx TransformContext) GetResourceSchemaURLItem() ctxcommon.SchemaURLItem {
	return tCtx.resourceProfiles
}

// NewParser creates a new profile parser with the provided functions and options.
func NewParser(functions map[string]ottl.Factory[TransformContext], telemetrySettings component.TelemetrySettings, options ...ottl.Option[TransformContext]) (ottl.Parser[TransformContext], error) {
	return ctxcommon.NewParser(
		functions,
		telemetrySettings,
		pathExpressionParser(getCache),
		parseEnum,
		options...,
	)
}

// EnablePathContextNames enables the support for path's context names on statements.
// When this option is configured, all statement's paths must have a valid context prefix,
// otherwise an error is reported.
//
// Experimental: *NOTE* this option is subject to change or removal in the future.
func EnablePathContextNames() ottl.Option[TransformContext] {
	return func(p *ottl.Parser[TransformContext]) {
		ottl.WithPathContextNames[TransformContext]([]string{
			ctxprofilestack.Name,
			ctxscope.LegacyName,
			ctxscope.Name,
			ctxresource.Name,
		})(p)
	}
}

// StatementSequenceOption represents an option for configuring a statement sequence.
type StatementSequenceOption func(*ottl.StatementSequence[TransformContext])

// WithStatementSequenceErrorMode sets the error mode for a statement sequence.
func WithStatementSequenceErrorMode(errorMode ottl.ErrorMode) StatementSequenceOption {
	return func(s *ottl.StatementSequence[TransformContext]) {
		ottl.WithStatementSequenceErrorMode[TransformContext](errorMode)(s)
	}
}

// NewStatementSequence creates a new statement sequence with the provided statements and options.
func NewStatementSequence(statements []*ottl.Statement[TransformContext], telemetrySettings component.TelemetrySettings, options ...StatementSequenceOption) ottl.StatementSequence[TransformContext] {
	s := ottl.NewStatementSequence(statements, telemetrySettings)
	for _, op := range options {
		op(&s)
	}
	return s
}

// ConditionSequenceOption represents an option for configuring a condition sequence.
type ConditionSequenceOption func(*ottl.ConditionSequence[TransformContext])

// WithConditionSequenceErrorMode sets the error mode for a condition sequence.
func WithConditionSequenceErrorMode(errorMode ottl.ErrorMode) ConditionSequenceOption {
	return func(c *ottl.ConditionSequence[TransformContext]) {
		ottl.WithConditionSequenceErrorMode[TransformContext](errorMode)(c)
	}
}

// NewConditionSequence creates a new condition sequence with the provided conditions and options.
func NewConditionSequence(conditions []*ottl.Condition[TransformContext], telemetrySettings component.TelemetrySettings, options ...ConditionSequenceOption) ottl.ConditionSequence[TransformContext] {
	c := ottl.NewConditionSequence(conditions, telemetrySettings)
	for _, op := range options {
		op(&c)
	}
	return c
}

func parseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	if val != nil {
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, errors.New("enum symbol not provided")
}

func getCache(tCtx TransformContext) pcommon.Map {
	return tCtx.cache
}

func pathExpressionParser(cacheGetter ctxcache.Getter[TransformContext]) ottl.PathExpressionParser[TransformContext] {
	return ctxcommon.PathExpressionParser(
		ctxprofilestack.Name,
		ctxprofilestack.DocRef,
		cacheGetter,
		map[string]ottl.PathExpressionParser[TransformContext]{
			ctxresource.Name:      ctxresource.PathGetSetter[TransformContext],
			ctxscope.Name:         ctxscope.PathGetSetter[TransformContext],
			ctxscope.LegacyName:   ctxscope.PathGetSetter[TransformContext],
			ctxprofile.Name:       ctxprofile.PathGetSetter[TransformContext],
			ctxprofilesample.Name: ctxprofilesample.PathGetSetter[TransformContext],
			ctxprofilestack.Name:  ctxprofilestack.PathGetSetter[TransformContext],
		})
}
