// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxmetadata"

import (
	"context"
	"errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"go.opentelemetry.io/collector/client"
)

const (
	Name = "metadata"
)

func PathExpressionParser[K any]() ottl.PathExpressionParser[K] {
	return func(path ottl.Path[K]) (ottl.GetSetter[K], error) {
		if path.Keys() == nil {
			return accessMetadata[K](), nil
		}
		return accessMetadataKey[K](path.Keys()), nil
	}
}

func accessMetadata[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			cl := client.FromContext(ctx)
			return cl.Metadata, nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return errors.New("cannot set value in metadata")
		},
	}
}

func accessMetadataKey[K any](keys []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			if len(keys) == 0 {
				return nil, errors.New("cannot get map value without keys")
			}
			cl := client.FromContext(ctx)
			key, err := ctxutil.GetMapKeyName(ctx, tCtx, keys[0])
			if err != nil {
				return nil, err
			}
			ss := cl.Metadata.Get(*key)

			if len(ss) != 1 {
				return nil, nil
			}

			return ss[0], nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			return errors.New("cannot set value in metadata")
		},
	}
}
