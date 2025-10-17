// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type EncodeArguments[K any] struct {
	Target   ottl.Getter[K]
	Encoding string
}

func NewEncodeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Encode", &EncodeArguments[K]{}, createEncodeFunction[K])
}

func createEncodeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*EncodeArguments[K])
	if !ok {
		return nil, errors.New("EncodeFactory args must be of type *EncodeArguments[K]")
	}

	return Encode(args.Target, args.Encoding)
}

func Encode[K any](target ottl.Getter[K], encoding string) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		var stringValue string

		switch v := val.(type) {
		case []byte:
			stringValue = string(v)
		case *string:
			stringValue = *v
		case string:
			stringValue = v
		case pcommon.ByteSlice:
			stringValue = string(v.AsRaw())
		case *pcommon.ByteSlice:
			stringValue = string(v.AsRaw())
		case pcommon.Value:
			stringValue = v.AsString()
		case *pcommon.Value:
			stringValue = v.AsString()
		default:
			return nil, fmt.Errorf("unsupported type provided to Encode function: %T", v)
		}

		switch encoding {
		// base64 is not in IANA index, so we have to deal with this encoding separately
		case "base64":
			return base64.StdEncoding.EncodeToString([]byte(stringValue)), nil
		case "base64-raw":
			return base64.RawStdEncoding.EncodeToString([]byte(stringValue)), nil
		case "base64-url":
			return base64.URLEncoding.EncodeToString([]byte(stringValue)), nil
		case "base64-raw-url":
			return base64.RawURLEncoding.EncodeToString([]byte(stringValue)), nil
		default:
			e, err := textutils.LookupEncoding(encoding)
			if err != nil {
				return nil, err
			}

			encodedString, err := e.NewEncoder().String(stringValue)
			if err != nil {
				return nil, fmt.Errorf("could not encode: %w", err)
			}

			return encodedString, nil
		}
	}, nil
}
