// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilesample // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilesample"

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilecommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

var (
	errMaxValueExceed   = errors.New("exceeded max value")
	errInvalidValueType = errors.New("invalid value type")
)

func PathGetSetter[K Context](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	if path == nil {
		return nil, ctxerror.New("nil", "nil", Name, DocRef)
	}
	switch path.Name() {
	case "locations_start_index":
		return accessLocationsStartIndex[K](), nil
	case "locations_length":
		return accessLocationsLength[K](), nil
	case "values":
		return accessValues[K](), nil
	case "attribute_indices":
		return accessAttributeIndices[K](), nil
	case "link_index":
		return accessLinkIndex[K](), nil
	case "timestamps_unix_nano":
		return accessTimestampsUnixNano[K](), nil
	case "timestamps":
		return accessTimestamps[K](), nil
	case "attributes":
		if path.Keys() == nil {
			return ctxprofilecommon.AccessAttributes[K](), nil
		}
		return ctxprofilecommon.AccessAttributesKey[K](path.Keys()), nil
	default:
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
}

func accessLocationsStartIndex[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return int64(tCtx.GetProfileSample().LocationsStartIndex()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(int64); ok {
				if v >= math.MaxInt32 {
					return errMaxValueExceed
				}
				tCtx.GetProfileSample().SetLocationsStartIndex(int32(v))
				return nil
			}
			return errInvalidValueType
		},
	}
}

func accessLocationsLength[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return int64(tCtx.GetProfileSample().LocationsLength()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(int64); ok {
				if v >= math.MaxInt32 {
					return errMaxValueExceed
				}
				tCtx.GetProfileSample().SetLocationsLength(int32(v))
				return nil
			}
			return errInvalidValueType
		},
	}
}

func accessValues[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[int64](tCtx.GetProfileSample().Value()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[int64](tCtx.GetProfileSample().Value(), val)
		},
	}
}

func accessAttributeIndices[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[int32](tCtx.GetProfileSample().AttributeIndices()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[int32](tCtx.GetProfileSample().AttributeIndices(), val)
		},
	}
}

func accessLinkIndex[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return int64(tCtx.GetProfileSample().LinkIndex()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if v, ok := val.(int64); ok {
				if v >= math.MaxInt32 {
					return errMaxValueExceed
				}
				tCtx.GetProfileSample().SetLinkIndex(int32(v))
				return nil
			}
			return errInvalidValueType
		},
	}
}

func accessTimestampsUnixNano[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return ctxutil.GetCommonIntSliceValues[uint64](tCtx.GetProfileSample().TimestampsUnixNano()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetCommonIntSliceValues[uint64](tCtx.GetProfileSample().TimestampsUnixNano(), val)
		},
	}
}

func accessTimestamps[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			var ts []time.Time
			for _, t := range tCtx.GetProfileSample().TimestampsUnixNano().All() {
				ts = append(ts, time.Unix(0, int64(t)).UTC())
			}
			return ts, nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if ts, ok := val.([]time.Time); ok {
				tCtx.GetProfileSample().TimestampsUnixNano().FromRaw([]uint64{})
				for _, t := range ts {
					tCtx.GetProfileSample().TimestampsUnixNano().Append(uint64(t.UTC().UnixNano()))
				}
			}
			return nil
		},
	}
}
