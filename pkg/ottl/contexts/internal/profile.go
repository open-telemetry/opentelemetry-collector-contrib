// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	ProfileContextName = "profile"
)

type ProfileContext interface {
	GetProfile() pprofile.Profile
}

func ProfilePathGetSetter[K ProfileContext](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	if path == nil {
		return nil, FormatDefaultErrorMessage(ProfileContextName, ProfileContextName, "Profile", ProfileRef)
	}
	switch path.Name() {
	case "sample_type":
		return accessSampleType[K](), nil
	case "sample":
		return accessSample[K](), nil
	case "mapping_table":
		return accessMappingTable[K](), nil
	case "location_table":
		return accessLocationTable[K](), nil
	case "location_indices":
		return accessLocationIndices[K](), nil
	case "function_table":
		return accessFunctionTable[K](), nil
	case "attribute_table":
		return accessAttributeTable[K](), nil
	case "attribute_units":
		return accessAttributeUnits[K](), nil
	case "link_table":
		return accessLinkTable[K](), nil
	case "string_table":
		return accessStringTable[K](), nil
	case "time_unix_nano":
		return accessTimeUnixNano[K](), nil
	case "time":
		return accessTime[K](), nil
	case "duration":
		return accessDuration[K](), nil
	case "period_type":
		return accessPeriodType[K](), nil
	case "period":
		return accessPeriod[K](), nil
	case "comment_string_indices":
		return accessCommentStringIndices[K](), nil
	case "default_sample_type_string_index":
		return accessDefaultSampleTypeStringIndex[K](), nil
	case "profile_id":
		return accessProfileID[K](), nil
	case "attribute_indices":
		return accessAttributeIndices[K](), nil
	case "dropped_attributes_count":
		return accessDroppedAttributesCount[K](), nil
	case "original_payload_format":
		return accessOriginalPayloadFormat[K](), nil
	case "original_payload":
		return accessOriginalPayload[K](), nil
	default:
		return nil, FormatDefaultErrorMessage(path.Name(), path.String(), "Profile", ProfileRef)
	}
}

func accessSampleType[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().SampleType(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.ValueTypeSlice); ok {
				v.CopyTo(tCtx.GetProfile().SampleType())
			}
			return nil
		},
	}
}

func accessSample[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().Sample(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.SampleSlice); ok {
				v.CopyTo(tCtx.GetProfile().Sample())
			}
			return nil
		},
	}
}

func accessMappingTable[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().MappingTable(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.MappingSlice); ok {
				v.CopyTo(tCtx.GetProfile().MappingTable())
			}
			return nil
		},
	}
}

func accessLocationTable[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().LocationTable(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.LocationSlice); ok {
				v.CopyTo(tCtx.GetProfile().LocationTable())
			}
			return nil
		},
	}
}

func accessLocationIndices[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().LocationIndices(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pcommon.Int32Slice); ok {
				v.CopyTo(tCtx.GetProfile().LocationIndices())
			}
			return nil
		},
	}
}

func accessFunctionTable[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().FunctionTable(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.FunctionSlice); ok {
				v.CopyTo(tCtx.GetProfile().FunctionTable())
			}
			return nil
		},
	}
}

func accessAttributeTable[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().AttributeTable(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.AttributeTableSlice); ok {
				v.CopyTo(tCtx.GetProfile().AttributeTable())
			}
			return nil
		},
	}
}

func accessAttributeUnits[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().AttributeUnits(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.AttributeUnitSlice); ok {
				v.CopyTo(tCtx.GetProfile().AttributeUnits())
			}
			return nil
		},
	}
}

func accessLinkTable[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().LinkTable(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.LinkSlice); ok {
				v.CopyTo(tCtx.GetProfile().LinkTable())
			}
			return nil
		},
	}
}

func accessStringTable[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().StringTable(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pcommon.StringSlice); ok {
				v.CopyTo(tCtx.GetProfile().StringTable())
			}
			return nil
		},
	}
}

func accessTimeUnixNano[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().Time().AsTime().UnixNano(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if i, ok := val.(int64); ok {
				tCtx.GetProfile().SetTime(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
			return nil
		},
	}
}

func accessTime[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().Time().AsTime(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if i, ok := val.(time.Time); ok {
				tCtx.GetProfile().SetTime(pcommon.NewTimestampFromTime(i))
			}
			return nil
		},
	}
}

func accessDuration[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().Duration(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if i, ok := val.(pcommon.Timestamp); ok {
				tCtx.GetProfile().SetDuration(i)
			}
			return nil
		},
	}
}

func accessPeriodType[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().PeriodType(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pprofile.ValueType); ok {
				v.CopyTo(tCtx.GetProfile().PeriodType())
			}
			return nil
		},
	}
}

func accessPeriod[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().Period(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if i, ok := val.(int64); ok {
				tCtx.GetProfile().SetPeriod(i)
			}
			return nil
		},
	}
}

func accessCommentStringIndices[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().CommentStrindices(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pcommon.Int32Slice); ok {
				v.CopyTo(tCtx.GetProfile().CommentStrindices())
			}
			return nil
		},
	}
}

func accessDefaultSampleTypeStringIndex[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().DefaultSampleTypeStrindex(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if i, ok := val.(int32); ok {
				tCtx.GetProfile().SetDefaultSampleTypeStrindex(i)
			}
			return nil
		},
	}
}

func accessProfileID[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().ProfileID(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if i, ok := val.(pprofile.ProfileID); ok {
				tCtx.GetProfile().SetProfileID(i)
			}
			return nil
		},
	}
}

func accessAttributeIndices[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().AttributeIndices(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pcommon.Int32Slice); ok {
				v.CopyTo(tCtx.GetProfile().AttributeIndices())
			}
			return nil
		},
	}
}

func accessDroppedAttributesCount[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().DroppedAttributesCount(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if i, ok := val.(uint32); ok {
				tCtx.GetProfile().SetDroppedAttributesCount(i)
			}
			return nil
		},
	}
}

func accessOriginalPayloadFormat[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().OriginalPayloadFormat(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(string); ok {
				tCtx.GetProfile().SetOriginalPayloadFormat(v)
			}
			return nil
		},
	}
}

func accessOriginalPayload[K ProfileContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetProfile().OriginalPayload(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(pcommon.ByteSlice); ok {
				v.CopyTo(tCtx.GetProfile().OriginalPayload())
			}
			return nil
		},
	}
}
