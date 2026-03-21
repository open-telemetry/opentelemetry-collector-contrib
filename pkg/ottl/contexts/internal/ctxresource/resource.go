// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxresource // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxresource"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/xpdata/entity"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

func PathGetSetter[K Context](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	if path == nil {
		return nil, ctxerror.New("nil", "nil", Name, DocRef)
	}
	switch path.Name() {
	case "attributes":
		if path.Keys() == nil {
			return accessResourceAttributes[K](), nil
		}
		return accessResourceAttributesKey[K](path.Keys()), nil
	case "dropped_attributes_count":
		return accessResourceDroppedAttributesCount[K](), nil
	case "schema_url":
		return accessResourceSchemaURLItem[K](), nil
	default:
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
}

func accessResourceAttributes[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetResource().Attributes(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			err := ctxutil.SetMap(tCtx.GetResource().Attributes(), val)
			if err != nil {
				return err
			}
			syncEntityRefs(tCtx.GetResource())
			return nil
		},
	}
}

func accessResourceAttributesKey[K Context](keys []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			return ctxutil.GetMapValue[K](ctx, tCtx, tCtx.GetResource().Attributes(), keys)
		},
		Setter: func(ctx context.Context, tCtx K, val any) error {
			return ctxutil.SetMapValue[K](ctx, tCtx, tCtx.GetResource().Attributes(), keys, val)
		},
	}
}

func accessResourceDroppedAttributesCount[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return int64(tCtx.GetResource().DroppedAttributesCount()), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if i, ok := val.(int64); ok {
				tCtx.GetResource().SetDroppedAttributesCount(uint32(i))
			}
			return nil
		},
	}
}

func accessResourceSchemaURLItem[K Context]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(_ context.Context, tCtx K) (any, error) {
			return tCtx.GetResourceSchemaURLItem().SchemaUrl(), nil
		},
		Setter: func(_ context.Context, tCtx K, val any) error {
			if schemaURL, ok := val.(string); ok {
				tCtx.GetResourceSchemaURLItem().SetSchemaUrl(schemaURL)
			}
			return nil
		},
	}
}

func syncEntityRefs(resource pcommon.Resource) {
	entityRefs := entity.ResourceEntityRefs(resource)
	if entityRefs.Len() == 0 {
		return
	}
	attrs := resource.Attributes()
	for i := entityRefs.Len() - 1; i >= 0; i-- {
		entityRef := entityRefs.At(i)
		missingIDKey := false
		for _, IDKey := range entityRef.IdKeys().All() {
			if _, exist := attrs.Get(IDKey); !exist {
				missingIDKey = true
				break
			}
		}
		if missingIDKey {
			entityRefs.RemoveIf(func(e entity.EntityRef) bool {
				return e.Type() == entityRef.Type()
			})
			continue
		}
		for _, descKey := range entityRef.DescriptionKeys().All() {
			if _, exist := attrs.Get(descKey); !exist {
				entityRef.DescriptionKeys().RemoveIf(func(dk string) bool {
					return dk == descKey
				})
			}
		}
	}
}
