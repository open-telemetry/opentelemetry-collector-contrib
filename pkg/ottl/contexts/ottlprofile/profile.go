// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprofile // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"
)

const (
	// Experimental: *NOTE* this constant is subject to change or removal in the future.
	ContextName            = "profile"
	contextNameDescription = "Profile"
)

var (
	_ internal.ResourceContext             = (*TransformContext)(nil)
	_ internal.InstrumentationScopeContext = (*TransformContext)(nil)
	//	_ zapcore.ObjectMarshaler              = (*TransformContext)(nil)
)

type TransformContext struct {
	profile              pprofile.Profile
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
	cache                pcommon.Map
	scopeProfiles        pprofile.ScopeProfiles
	resourceProfiles     pprofile.ResourceProfiles
}

type Option func(*ottl.Parser[TransformContext])

type TransformContextOption func(*TransformContext)

func NewTransformContext(profile pprofile.Profile, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource, scopeProfiles pprofile.ScopeProfiles, resourceProfiles pprofile.ResourceProfiles, options ...TransformContextOption) TransformContext {
	tc := TransformContext{
		profile:              profile,
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

// Experimental: *NOTE* this option is subject to change or removal in the future.
func WithCache(cache *pcommon.Map) TransformContextOption {
	return func(p *TransformContext) {
		if cache != nil {
			p.cache = *cache
		}
	}
}

func (tCtx TransformContext) GetProfile() pprofile.Profile {
	return tCtx.profile
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
	return tCtx.scopeProfiles
}

func (tCtx TransformContext) GetResourceSchemaURLItem() internal.SchemaURLItem {
	return tCtx.resourceProfiles
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
			ContextName,
			internal.InstrumentationScopeContextName,
			internal.ResourceContextName,
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

// symbolTable and parseEnum are here for completeness, currently unused.
var symbolTable = map[ottl.EnumSymbol]ottl.Enum{}

func parseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	if val != nil {
		if enum, ok := symbolTable[*val]; ok {
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
		return nil, fmt.Errorf("path cannot be nil")
	}
	// Higher contexts parsing
	if path.Context() != "" && path.Context() != ContextName {
		return pep.parseHigherContextPath(path.Context(), path)
	}
	// Backward compatibility with paths without context
	if path.Context() == "" && (path.Name() == internal.ResourceContextName || path.Name() == internal.InstrumentationScopeContextName) {
		return pep.parseHigherContextPath(path.Name(), path.Next())
	}

	switch path.Name() {
	case "cache":
		if path.Keys() == nil {
			return accessCache(), nil
		}
		return accessCacheKey(path.Keys()), nil
	case "sample_type":
		return accessSampleType(), nil
	case "sample":
		return accessSample(), nil
	case "mapping_table":
		return accessMappingTable(), nil
	case "location_table":
		return accessLocationTable(), nil
	case "location_indices":
		return accessLocationIndices(), nil
	case "function_table":
		return accessFunctionTable(), nil
	case "attribute_table":
		return accessAttributeTable(), nil
	case "attribute_units":
		return accessAttributeUnits(), nil
	case "link_table":
		return accessLinkTable(), nil
	case "string_table":
		return accessStringTable(), nil
	case "time_unix_nano":
		return accessTimeUnixNano(), nil
	case "time":
		return accessTime(), nil
	case "duration":
		return accessDuration(), nil
	case "period_type":
		return accessPeriodType(), nil
	case "period":
		return accessPeriod(), nil
	case "comment_string_indices":
		return accessCommentStringIndices(), nil
	case "default_sample_type_string_index":
		return accessDefaultSampleTypeStringIndex(), nil
	case "profile_id":
		return accessProfileID(), nil
	case "attribute_indices":
		return accessAttributeIndices(), nil
	case "dropped_attributes_count":
		return accessDroppedAttributesCount(), nil
	case "original_payload_format":
		return accessOriginalPayloadFormat(), nil
	case "original_payload":
		return accessOriginalPayload(), nil
	default:
		return nil, internal.FormatDefaultErrorMessage(path.Name(), path.String(), contextNameDescription, internal.ProfileRef)
	}
}

func (pep *pathExpressionParser) parseHigherContextPath(context string, path ottl.Path[TransformContext]) (ottl.GetSetter[TransformContext], error) {
	switch context {
	case internal.ResourceContextName:
		return internal.ResourcePathGetSetter(ContextName, path)
	case internal.InstrumentationScopeContextName:
		return internal.ScopePathGetSetter(ContextName, path)
	default:
		var fullPath string
		if path != nil {
			fullPath = path.String()
		}
		return nil, internal.FormatDefaultErrorMessage(context, fullPath, contextNameDescription, internal.ProfileRef)
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
			return internal.GetMapValue[TransformContext](ctx, tCtx, tCtx.getCache(), key)
		},
		Setter: func(ctx context.Context, tCtx TransformContext, val any) error {
			return internal.SetMapValue[TransformContext](ctx, tCtx, tCtx.getCache(), key, val)
		},
	}
}

func accessSampleType() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().SampleType(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.ValueTypeSlice); ok {
				v.CopyTo(tCtx.GetProfile().SampleType())
			}
			return nil
		},
	}
}

func accessSample() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().Sample(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.SampleSlice); ok {
				v.CopyTo(tCtx.GetProfile().Sample())
			}
			return nil
		},
	}
}

func accessMappingTable() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().MappingTable(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.MappingSlice); ok {
				v.CopyTo(tCtx.GetProfile().MappingTable())
			}
			return nil
		},
	}
}

func accessLocationTable() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().LocationTable(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.LocationSlice); ok {
				v.CopyTo(tCtx.GetProfile().LocationTable())
			}
			return nil
		},
	}
}

func accessLocationIndices() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().LocationIndices(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pcommon.Int32Slice); ok {
				v.CopyTo(tCtx.GetProfile().LocationIndices())
			}
			return nil
		},
	}
}

func accessFunctionTable() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().FunctionTable(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.FunctionSlice); ok {
				v.CopyTo(tCtx.GetProfile().FunctionTable())
			}
			return nil
		},
	}
}

func accessAttributeTable() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().AttributeTable(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.AttributeTableSlice); ok {
				v.CopyTo(tCtx.GetProfile().AttributeTable())
			}
			return nil
		},
	}
}

func accessAttributeUnits() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().AttributeUnits(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.AttributeUnitSlice); ok {
				v.CopyTo(tCtx.GetProfile().AttributeUnits())
			}
			return nil
		},
	}
}

func accessLinkTable() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().LinkTable(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.LinkSlice); ok {
				v.CopyTo(tCtx.GetProfile().LinkTable())
			}
			return nil
		},
	}
}

func accessStringTable() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().StringTable(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pcommon.StringSlice); ok {
				v.CopyTo(tCtx.GetProfile().StringTable())
			}
			return nil
		},
	}
}

func accessTimeUnixNano() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().Time().AsTime().UnixNano(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if i, ok := val.(int64); ok {
				tCtx.GetProfile().SetTime(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
			return nil
		},
	}
}

func accessTime() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().Time().AsTime(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if i, ok := val.(time.Time); ok {
				tCtx.GetProfile().SetTime(pcommon.NewTimestampFromTime(i))
			}
			return nil
		},
	}
}

func accessDuration() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().Duration(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if i, ok := val.(pcommon.Timestamp); ok {
				tCtx.GetProfile().SetDuration(i)
			}
			return nil
		},
	}
}

func accessPeriodType() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().PeriodType(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pprofile.ValueType); ok {
				v.CopyTo(tCtx.GetProfile().PeriodType())
			}
			return nil
		},
	}
}

func accessPeriod() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().Period(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if i, ok := val.(int64); ok {
				tCtx.GetProfile().SetPeriod(i)
			}
			return nil
		},
	}
}

func accessCommentStringIndices() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().CommentStrindices(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pcommon.Int32Slice); ok {
				v.CopyTo(tCtx.GetProfile().CommentStrindices())
			}
			return nil
		},
	}
}

func accessDefaultSampleTypeStringIndex() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().DefaultSampleTypeStrindex(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if i, ok := val.(int32); ok {
				tCtx.GetProfile().SetDefaultSampleTypeStrindex(i)
			}
			return nil
		},
	}
}

func accessProfileID() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().ProfileID(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if i, ok := val.(pprofile.ProfileID); ok {
				tCtx.GetProfile().SetProfileID(i)
			}
			return nil
		},
	}
}

func accessAttributeIndices() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().AttributeIndices(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pcommon.Int32Slice); ok {
				v.CopyTo(tCtx.GetProfile().AttributeIndices())
			}
			return nil
		},
	}
}

func accessDroppedAttributesCount() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().DroppedAttributesCount(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if i, ok := val.(uint32); ok {
				tCtx.GetProfile().SetDroppedAttributesCount(i)
			}
			return nil
		},
	}
}

func accessOriginalPayloadFormat() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().OriginalPayloadFormat(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(string); ok {
				tCtx.GetProfile().SetOriginalPayloadFormat(v)
			}
			return nil
		},
	}
}

func accessOriginalPayload() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(_ context.Context, tCtx TransformContext) (any, error) {
			return tCtx.GetProfile().OriginalPayload(), nil
		},
		Setter: func(_ context.Context, tCtx TransformContext, val any) error {
			if v, ok := val.(pcommon.ByteSlice); ok {
				v.CopyTo(tCtx.GetProfile().OriginalPayload())
			}
			return nil
		},
	}
}
