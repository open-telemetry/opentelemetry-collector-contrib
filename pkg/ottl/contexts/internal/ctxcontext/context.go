// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcontext // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxcontext"

import (
	"context"
	"encoding/json"
	"errors"

	"go.opentelemetry.io/collector/client"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxerror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
)

const (
	Name   = "context"
	DocRef = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/internal/ctxcontext"
)

func PathExpressionParser[K any]() ottl.PathExpressionParser[K] {
	return func(path ottl.Path[K]) (ottl.GetSetter[K], error) {
		nextPath := path.Next()
		if nextPath == nil {
			return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
		}
		switch nextPath.Name() {
		case "client":
			return accessClientContext[K](nextPath)
		case "grpc":
			return accessGrpcContext[K](nextPath)
		default:
			return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
		}
	}
}

func accessGrpcContext[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath == nil {
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
	switch nextPath.Name() {
	case "metadata":
		if nextPath.Keys() == nil {
			return accessGrpcMetadataContext[K](), nil
		}
		return accessGrpcMetadataContextKey[K](nextPath.Keys()), nil
	default:
		return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
	}
}

func accessClientContext[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath == nil {
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
	switch nextPath.Name() {
	case "addr":
		return accessAddrContext(nextPath)
	case "auth":
		return accessAuthContext(nextPath)
	case "metadata":
		return accessMetadataContext(nextPath)
	default:
		return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
	}
}

func accessMetadataContext[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath != nil {
		return nil, ctxerror.New(nextPath.Name(), nextPath.String(), Name, DocRef)
	}
	if path.Keys() == nil {
		return accessClientMetadata[K](), nil
	}
	return accessClientMetadataKey[K](path.Keys()), nil
}

func accessAddrContext[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
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

func accessGrpcMetadataContext[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				return nil, nil
			}
			return md, nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.grpc.metadata")
		},
	}
}

func accessGrpcMetadataContextKey[K any](keys []ottl.Key[K]) ottl.StandardGetSetter[K] {
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
			attrVal := md.Get(*key)
			return attrVal, nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.grpc.metadata")
		},
	}
}

func accessAuthContext[K any](path ottl.Path[K]) (ottl.GetSetter[K], error) {
	nextPath := path.Next()
	if nextPath == nil {
		return nil, ctxerror.New(path.Name(), path.String(), Name, DocRef)
	}
	switch nextPath.Name() {
	case "attributes":
		if nextPath.Keys() == nil {
			return accessAuthAttributes[K](), nil
		}
		return accessAuthAttributesKey[K](nextPath.Keys()), nil
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

func accessAuthAttributes[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			cl := client.FromContext(ctx)
			attrMap := make(map[string]string)
			names := cl.Auth.GetAttributeNames()
			for _, name := range names {
				attrVal := cl.Auth.GetAttribute(name)
				attrStr, err := getAuthAttributeValue(attrVal)
				if err != nil {
					return nil, err
				}
				attrMap[name] = attrStr
			}
			return attrMap, nil
		},
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.client.auth.attributes")
		},
	}
}

func accessAuthAttributesKey[K any](keys []ottl.Key[K]) ottl.StandardGetSetter[K] {
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

func accessClientMetadata[K any]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx context.Context, _ K) (any, error) {
			cl := client.FromContext(ctx)
			return cl.Metadata, nil
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
		Setter: func(_ context.Context, _ K, _ any) error {
			return errors.New("cannot set value in context.client.metadata")
		},
	}
}
