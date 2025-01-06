// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/twmb/murmur3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type murmur3Variant int

const (
	Murmur3Sum32 murmur3Variant = iota
	Murmur3Sum128
	Murmur3Hex32
	Murmur3Hex128
)

type Murmur3Arguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewMurmur3HashFactory[K any]() ottl.Factory[K] {
	return NewMurmur3Factory[K]("Murmur3Hash", Murmur3Sum32)
}

func NewMurmur3Hash128Factory[K any]() ottl.Factory[K] {
	return NewMurmur3Factory[K]("Murmur3Hash128", Murmur3Sum128)
}

func NewMurmur3HexFactory[K any]() ottl.Factory[K] {
	return NewMurmur3Factory[K]("Murmur3Hex", Murmur3Hex32)
}

func NewMurmur3Hex128Factory[K any]() ottl.Factory[K] {
	return NewMurmur3Factory[K]("Murmur3Hex128", Murmur3Hex128)
}

func NewMurmur3Factory[K any](name string, variant murmur3Variant) ottl.Factory[K] {
	return ottl.NewFactory(name, &Murmur3Arguments[K]{}, func(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
		return createMurmur3Function[K](name, oArgs, variant)
	})
}

func createMurmur3Function[K any](name string, oArgs ottl.Arguments, variant murmur3Variant) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*Murmur3Arguments[K])
	if !ok {
		return nil, fmt.Errorf("%s args must be of type *Murmur3Arguments[K]", name)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := args.Target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch variant {
		case Murmur3Sum32:
			h := murmur3.Sum32([]byte(val))
			return int64(h), nil
		case Murmur3Sum128:
			h1, h2 := murmur3.Sum128([]byte(val))
			return []int64{int64(h1), int64(h2)}, nil
		// MurmurHash3 is sensitive to endianness
		// Hex returns the hexadecimal representation of the hash in little-endian
		case Murmur3Hex32:
			h := murmur3.Sum32([]byte(val))
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, h)
			return hex.EncodeToString(b), nil
		case Murmur3Hex128:
			h1, h2 := murmur3.Sum128([]byte(val))
			b := make([]byte, 16)
			binary.LittleEndian.PutUint64(b[:8], h1)
			binary.LittleEndian.PutUint64(b[8:], h2)
			return hex.EncodeToString(b), nil
		default:
			return nil, fmt.Errorf("unknown murmur3 variant: %d", variant)
		}
	}, nil
}
