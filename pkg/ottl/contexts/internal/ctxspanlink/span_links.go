// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxspanlink // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxspanlink"

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

func PathGetSetter[K Context](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	if path == nil {
		return nil, ctxerror.New("nil", "nil", Name, DocRef)
	}
	switch path.Name() {
	case "trace_id":
		nextPath := path.Next()
		if nextPath != nil {
			if nextPath.Name() == "string" {
				return accessStringTraceID[K](), nil
			}
			return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
		}
		return accessTraceID[K](), nil
	case "span_id":
		nextPath := path.Next()
		if nextPath != nil {
			if nextPath.Name() == "string" {
				return accessStringSpanID[K](), nil
			}
			return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
		}
		return accessSpanID[K](), nil
	case "trace_state":
		return accessTraceState[K](), nil
	case "attributes":
		if path.Keys() == nil {
			return accessAttributes[K](), nil
		}
		return accessAttributesKey(path.Keys()), nil
	case "dropped_attributes_count":
		return accessDroppedAttributesCount[K](), nil
	case "flags":
		return accessFlags[K](), nil
	default:
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
}

func accessTraceID[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetSpanLink().TraceID(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			newTraceID, err := ctxutil.ExpectType[pcommon.TraceID](val)
			if err != nil {
				return err
			}
			tCtx.GetSpanLink().SetTraceID(newTraceID)
			return nil
		},
	}
}

func accessStringTraceID[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			id := tCtx.GetSpanLink().TraceID()
			return hex.EncodeToString(id[:]), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			str, err := ctxutil.ExpectType[string](val)
			if err != nil {
				return err
			}
			id, err := ctxcommon.ParseTraceID(str)
			if err != nil {
				return err
			}
			tCtx.GetSpanLink().SetTraceID(id)
			return nil
		},
	}
}

func accessSpanID[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetSpanLink().SpanID(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			newSpanID, err := ctxutil.ExpectType[pcommon.SpanID](val)
			if err != nil {
				return err
			}
			tCtx.GetSpanLink().SetSpanID(newSpanID)
			return nil
		},
	}
}

func accessStringSpanID[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			id := tCtx.GetSpanLink().SpanID()
			return hex.EncodeToString(id[:]), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			str, err := ctxutil.ExpectType[string](val)
			if err != nil {
				return err
			}
			id, err := ctxcommon.ParseSpanID(str)
			if err != nil {
				return err
			}
			tCtx.GetSpanLink().SetSpanID(id)
			return nil
		},
	}
}

func accessTraceState[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetSpanLink().TraceState().AsRaw(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			str, err := ctxutil.ExpectType[string](val)
			if err != nil {
				return err
			}
			tCtx.GetSpanLink().TraceState().FromRaw(str)
			return nil
		},
	}
}

func accessAttributes[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetSpanLink().Attributes(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return ctxutil.SetMap(tCtx.GetSpanLink().Attributes(), val)
		},
	}
}

func accessAttributesKey[K Context](key []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			return ctxutil.GetMapValue[K](ctx, tCtx, tCtx.GetSpanLink().Attributes(), key)
		},
		Setter: func(ctx context.Context, tCtx K, val any) error {
			return ctxutil.SetMapValue[K](ctx, tCtx, tCtx.GetSpanLink().Attributes(), key, val)
		},
	}
}

func accessDroppedAttributesCount[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return int64(tCtx.GetSpanLink().DroppedAttributesCount()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			newCount, err := ctxutil.ExpectType[int64](val)
			if err != nil {
				return err
			}
			tCtx.GetSpanLink().SetDroppedAttributesCount(uint32(newCount))
			return nil
		},
	}
}

func accessFlags[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return int64(tCtx.GetSpanLink().Flags()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			value, err := ctxutil.ExpectType[int64](val)
			if err != nil {
				return err
			}
			if value < 0 || value > math.MaxUint32 {
				return fmt.Errorf("value %d is out of range for uint32", value)
			}
			tCtx.GetSpanLink().SetFlags(uint32(value))
			return nil
		},
	}
}
