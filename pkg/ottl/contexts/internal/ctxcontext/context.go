// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcontext // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcontext"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
)

const (
	readOnlyPathErrMsg = "%q is read-only and cannot be modified"
	Name               = "context"
	DocRef             = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlcontext"
)

func PathGetSetter[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	switch path.Name() {
	case "client":
		return accessClient[K](path)
	case "grpc":
		return accessGRPC[K](path)
	default:
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
}

func accessGRPC[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath == nil {
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
	switch nextPath.Name() {
	case "metadata":
		if nextPath.Keys() == nil {
			return accessGRPCMetadataKeys[K](), nil
		}
		return accessGRPCMetadataKey[K](nextPath.Keys()), nil
	default:
		return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
	}
}

func accessClient[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath == nil {
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
	switch nextPath.Name() {
	case "addr":
		return accessClientAddr(nextPath)
	case "auth":
		return accessClientAuth(nextPath)
	case "metadata":
		return accessClientMetadata(nextPath)
	default:
		return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
	}
}

func accessClientMetadata[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath != nil {
		return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
	}
	if path.Keys() == nil {
		return accessClientMetadataKeys[K](), nil
	}
	return accessClientMetadataKey[K](path.Keys()), nil
}

func accessClientAddr[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath != nil {
		return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
	}
	if path.Keys() != nil {
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			cl := client.FromContext(ctx)
			if cl.Addr == nil {
				return nil, nil
			}
			return cl.Addr.String(), nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return fmt.Errorf(readOnlyPathErrMsg, "context.client.addr")
		},
	}, nil
}

func convertStringArrToValueSlice(vals []string) pcommon.Value {
	val := pcommon.NewValueSlice()
	sl := val.Slice()
	sl.EnsureCapacity(len(vals))
	for _, val := range vals {
		sl.AppendEmpty().SetStr(val)
	}
	return val
}

func convertGRPCMetadataToMap(md metadata.MD) pcommon.Map {
	mdMap := pcommon.NewMap()
	mdMap.EnsureCapacity(len(md))
	for k, v := range md {
		convertStringArrToValueSlice(v).MoveTo(mdMap.PutEmpty(k))
	}
	return mdMap
}

func getIndexableValueFromStringArr[K any](ctx context.Context, tCtx K, keys []ottl.Key[K], strSlice []string) (any, error) {
	if len(keys) == 0 {
		slice := pcommon.NewSlice()
		slice.EnsureCapacity(len(strSlice))
		for _, str := range strSlice {
			slice.AppendEmpty().SetStr(str)
		}
		return slice, nil
	}
	if len(keys) > 1 {
		return nil, errors.New("cannot index into string slice more than once")
	}
	index, err := ctxutil.GetSliceIndexFromKeys(ctx, tCtx, len(strSlice), keys)
	if err != nil {
		return nil, err
	}
	return strSlice[index], nil
}

func accessGRPCMetadataKeys[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				return pcommon.NewMap(), nil
			}
			return convertGRPCMetadataToMap(md), nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return fmt.Errorf(readOnlyPathErrMsg, "context.grpc.metadata")
		},
	}
}

func accessGRPCMetadataKey[K any](keys []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			if len(keys) == 0 {
				return nil, errors.New("cannot get map value without keys")
			}
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				return nil, nil
			}
			key, err := ctxutil.GetMapKeyName(ctx, tCtx, keys[0])
			if err != nil {
				return nil, err
			}
			mdVal := md.Get(*key)
			if len(mdVal) == 0 {
				return nil, nil
			}
			return getIndexableValueFromStringArr(ctx, tCtx, keys[1:], mdVal)
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return fmt.Errorf(readOnlyPathErrMsg, "context.grpc.metadata")
		},
	}
}

func accessClientAuth[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath == nil {
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
	switch nextPath.Name() {
	case "attributes":
		if nextPath.Keys() == nil {
			return accessClientAuthAttributesKeys[K](), nil
		}
		return accessClientAuthAttributesKey[K](nextPath.Keys()), nil
	default:
		return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
	}
}

func getAuthAttributeValue(authData client.AuthData, key string) (pcommon.Value, error) {
	attrVal := authData.GetAttribute(key)
	switch typedAttrVal := attrVal.(type) {
	case string:
		return pcommon.NewValueStr(typedAttrVal), nil
	case []string:
		value := pcommon.NewValueSlice()
		slice := value.Slice()
		slice.EnsureCapacity(len(typedAttrVal))
		for _, str := range typedAttrVal {
			slice.AppendEmpty().SetStr(str)
		}
		return value, nil
	default:
		value := pcommon.NewValueEmpty()
		err := value.FromRaw(attrVal)
		if err != nil {
			return pcommon.Value{}, err
		}
		return value, nil
	}
}

func convertAuthDataToMap(authData client.AuthData) pcommon.Map {
	authMap := pcommon.NewMap()
	if authData == nil {
		return authMap
	}
	names := authData.GetAttributeNames()
	authMap.EnsureCapacity(len(names))
	for _, name := range names {
		newKeyValue := authMap.PutEmpty(name)
		if value, err := getAuthAttributeValue(authData, name); err == nil {
			value.MoveTo(newKeyValue)
		}
	}
	return authMap
}

func accessClientAuthAttributesKeys[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			cl := client.FromContext(ctx)
			return convertAuthDataToMap(cl.Auth), nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return fmt.Errorf(readOnlyPathErrMsg, "context.client.auth.attributes")
		},
	}
}

func accessClientAuthAttributesKey[K any](keys []ottl.Key[K]) ottl.StandardGetSetter[K] {
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
			if cl.Auth == nil {
				return nil, nil
			}
			attrVal, err := getAuthAttributeValue(cl.Auth, *key)
			if err != nil {
				return nil, err
			}
			if len(keys) > 1 {
				switch attrVal.Type() {
				case pcommon.ValueTypeSlice:
					return ctxutil.GetSliceValue[K](ctx, tCtx, attrVal.Slice(), keys[1:])
				case pcommon.ValueTypeMap:
					return ctxutil.GetMapValue[K](ctx, tCtx, attrVal.Map(), keys[1:])
				default:
					return nil, fmt.Errorf("attribute %q value is not indexable: %T", *key, attrVal.Type().String())
				}
			}
			return ottlcommon.GetValue(attrVal), nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return fmt.Errorf(readOnlyPathErrMsg, "context.client.auth.attributes")
		},
	}
}

func convertClientMetadataToMap(md client.Metadata) pcommon.Map {
	mdMap := pcommon.NewMap()
	for k := range md.Keys() {
		convertStringArrToValueSlice(md.Get(k)).MoveTo(mdMap.PutEmpty(k))
	}
	return mdMap
}

func accessClientMetadataKeys[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			cl := client.FromContext(ctx)
			return convertClientMetadataToMap(cl.Metadata), nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return fmt.Errorf(readOnlyPathErrMsg, "context.client.metadata")
		},
	}
}

func accessClientMetadataKey[K any](keys []ottl.Key[K]) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, tCtx K) (any, error) {
			if len(keys) == 0 {
				return nil, errors.New("cannot get map value without keys")
			}

			key, err := ctxutil.GetMapKeyName(ctx, tCtx, keys[0])
			if err != nil {
				return nil, fmt.Errorf("cannot get map value: %w", err)
			}
			cl := client.FromContext(ctx)
			mdVal := cl.Metadata.Get(*key)
			if len(mdVal) == 0 {
				return nil, nil
			}
			return getIndexableValueFromStringArr(ctx, tCtx, keys[1:], mdVal)
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return fmt.Errorf(readOnlyPathErrMsg, "context.client.metadata")
		},
	}
}
