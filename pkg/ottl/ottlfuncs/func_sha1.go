// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"crypto/sha1" // #nosec
	"encoding/hex"
	"fmt"
	"io"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SHA1Arguments[K any] struct {
	Target ottl.StringGetter[K] `ottlarg:"0"`
}

func NewSHA1Factory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SHA1", &SHA1Arguments[K]{}, createSHA1Function[K])
}

func createSHA1Function[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SHA1Arguments[K])

	if !ok {
		return nil, fmt.Errorf("SHA1Factory args must be of type *SHA1Arguments[K]")
	}

	return SHA1HashString(args.Target)
}

func SHA1HashString[K any](target ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {

	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if val == "" {
			return val, nil
		}

		hash := sha1.New() // #nosec
		if _, err := io.WriteString(hash, val); err != nil {
			return val, err
		}
		hashValue := hex.EncodeToString(hash.Sum(nil))
		return hashValue, nil
	}, nil
}
