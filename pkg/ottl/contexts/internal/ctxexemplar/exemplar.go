// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxexemplar // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxexemplar"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

func PathGetSetter[K Context](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	if path == nil {
		return nil, ctxerror.New("nil", "nil", Name, DocRef)
	}
	switch path.Name() {
	case "time_unix_nano":
		return accessExemplarTimeUnixNano[K](), nil
	case "time":
		return accessExemplarTime[K](), nil
	case "double_value":
		return accessExemplarDoubleValue[K](), nil
	case "int_value":
		return accessExemplarIntValue[K](), nil
	case "trace_id":
		return accessExemplarTraceID[K](), nil
	case "span_id":
		return accessExemplarSpanID[K](), nil
	case "filtered_attributes":
		if path.Keys() == nil {
			return accessExemplarFilteredAttributes[K](), nil
		}
		return accessExemplarFilteredAttributesKey(path.Keys()), nil
	default:
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
}

func accessExemplarTimeUnixNano[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetExemplar().Timestamp().AsTime().UnixNano(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if newTimestamp, ok := val.(int64); ok {
				tCtx.GetExemplar().SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTimestamp)))
			}
			return nil
		},
	}
}

func accessExemplarTime[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetExemplar().Timestamp().AsTime(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if newTimestamp, ok := val.(time.Time); ok {
				tCtx.GetExemplar().SetTimestamp(pcommon.NewTimestampFromTime(newTimestamp))
			}
			return nil
		},
	}
}

func accessExemplarDoubleValue[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetExemplar().DoubleValue(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if newVal, ok := val.(float64); ok {
				tCtx.GetExemplar().SetDoubleValue(newVal)
			}
			return nil
		},
	}
}

func accessExemplarIntValue[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetExemplar().IntValue(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if newVal, ok := val.(int64); ok {
				tCtx.GetExemplar().SetIntValue(newVal)
			}
			return nil
		},
	}
}

func accessExemplarTraceID[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetExemplar().TraceID(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if newTraceID, ok := val.(pcommon.TraceID); ok {
				tCtx.GetExemplar().SetTraceID(newTraceID)
			}
			return nil
		},
	}
}

func accessExemplarSpanID[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetExemplar().SpanID(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if newSpanID, ok := val.(pcommon.SpanID); ok {
				tCtx.GetExemplar().SetSpanID(newSpanID)
			}
			return nil
		},
	}
}

func accessExemplarFilteredAttributes[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetExemplar().FilteredAttributes(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetMap(tCtx.GetExemplar().FilteredAttributes(), val)
		},
	}
}

func accessExemplarFilteredAttributesKey[K Context](key []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			return ctxutil.GetMapValue[K](ctx, tCtx, tCtx.GetExemplar().FilteredAttributes(), key)
		},
		Setter: func(ctx context.Context, tCtx K, val any) error {
			return ctxutil.SetMapValue[K](ctx, tCtx, tCtx.GetExemplar().FilteredAttributes(), key, val)
		},
	}
}

// ExemplarValueType returns the value type of the exemplar.
// This is a convenience helper used by the public ottlexemplar context.
func ExemplarValueType(e pmetric.Exemplar) pmetric.ExemplarValueType {
	return e.ValueType()
}
