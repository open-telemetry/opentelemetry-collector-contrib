// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcontext // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcontext"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

const (
	Name   = "context"
	DocRef = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlcontext"
)

func PathExpressionParser[K any]() ottl.PathExpressionParser[K] {
	return func(path ottl.Path[K]) (ottl.GetSetter[K], error) {
		nextPath := path.Next()
		if nextPath == nil {
			return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
		}
		switch nextPath.Name() {
		case "client":
			return accessClient[K](nextPath)
		case "grpc":
			return accessGRPC[K](nextPath)
		default:
			return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
		}
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
			return cl.Addr.String(), nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.client.addr")
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
	for k, v := range md {
		sl := mdMap.PutEmptySlice(k)
		mdSl := convertStringArrToValueSlice(v)
		mdSl.Slice().CopyTo(sl)
	}
	return mdMap
}

func getIndexableValueFromStringArr[K any](ctx context.Context, tCtx K, keys []ottl.Key[K], strArr []string) (any, error) {
	val := convertStringArrToValueSlice(strArr)
	return ctxutil.GetIndexableValue[K](ctx, tCtx, val, keys[1:])
}

func accessGRPCMetadataKeys[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				return nil, nil
			}
			return convertGRPCMetadataToMap(md), nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.grpc.metadata")
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
			return getIndexableValueFromStringArr(ctx, tCtx, keys, mdVal)
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.grpc.metadata")
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

func getAuthAttributeValue(attr any) (string, error) {
	switch a := attr.(type) {
	case string:
		return a, nil
	case nil:
		return "", nil
	default:
		b, err := json.Marshal(attr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func convertAuthDataToMap(authData client.AuthData) (pcommon.Map, error) {
	authMap := pcommon.NewMap()
	names := authData.GetAttributeNames()
	for _, name := range names {
		attrVal := authData.GetAttribute(name)
		attrStr, err := getAuthAttributeValue(attrVal)
		if err != nil {
			return pcommon.NewMap(), err
		}
		authMap.PutStr(name, attrStr)
	}
	return authMap, nil
}

func accessClientAuthAttributesKeys[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			cl := client.FromContext(ctx)
			return convertAuthDataToMap(cl.Auth)
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.client.auth.attributes")
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
			attrVal := cl.Auth.GetAttribute(*key)
			attrStr, err := getAuthAttributeValue(attrVal)
			if err != nil {
				return nil, err
			}
			return attrStr, nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.client.auth.attributes")
		},
	}
}

func convertClientMetadataToMap(md client.Metadata) pcommon.Map {
	mdMap := pcommon.NewMap()
	for k := range md.Keys() {
		sl := mdMap.PutEmptySlice(k)
		mdVal := md.Get(k)
		mdSl := convertStringArrToValueSlice(mdVal)
		mdSl.Slice().CopyTo(sl)
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
			return errors.New("cannot set value in context.client.metadata")
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
			return getIndexableValueFromStringArr(ctx, tCtx, keys, mdVal)
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.client.metadata")
		},
	}
}
