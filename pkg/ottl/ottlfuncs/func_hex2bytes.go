// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type HexToBytesArguments[K any] struct {
	hexStr   string
}

func NewHexToBytesFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("HexToBytes", &HexToBytesArguments[K]{}, createHexToBytesFunction[K])
}

func createHexToBytesFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*HexToBytesArguments[K])
	if !ok {
		return nil, errors.New("HexToBytesFactory args must be of type *HexToBytesArguments[K]")
	}

	return HexToBytes[K](args.hexStr)
}

func HexToBytes[K any](hexStr string) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (any, error) {
		// Decode the hex string
		bytes, err := hex.DecodeString(hexStr)
		if err != nil {
			return nil, err
		}
		return bytes, nil
	}, nil
}
