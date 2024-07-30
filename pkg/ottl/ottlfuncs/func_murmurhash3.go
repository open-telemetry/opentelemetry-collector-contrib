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

const (
	v32Hash  = "v32_hash"
	v32Hex   = "v32_hex"
	v128Hash = "v128_hash" // default
	v128Hex  = "v128_hex"
)

type MurmurHash3Arguments[K any] struct {
	Target  ottl.StringGetter[K]
	Version ottl.Optional[string]
}

func NewMurmurHash3Factory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("MurmurHash3", &MurmurHash3Arguments[K]{}, createMurmurHash3Function[K])
}

func createMurmurHash3Function[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*MurmurHash3Arguments[K])

	if !ok {
		return nil, fmt.Errorf("MurmurHash3Factory args must be of type *MurmurHash3Arguments[K]")
	}

	version := v128Hash
	if !args.Version.IsEmpty() {
		v := args.Version.Get()

		switch v {
		case v32Hash, v32Hex, v128Hash, v128Hex:
			version = v
		default:
			return nil, fmt.Errorf("invalid arguments: %s", v)
		}
	}

	return murmurHash3(args.Target, version)
}

func murmurHash3[K any](target ottl.StringGetter[K], version string) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch version {
		case v32Hash:
			h := murmur3.Sum32([]byte(val))
			return int64(h), nil
		case v128Hash:
			h1, h2 := murmur3.Sum128([]byte(val))
			return []int64{int64(h1), int64(h2)}, nil
		case v32Hex, v128Hex:
			return hexStringLittleEndianVariant(val, version)
		default:
			return nil, fmt.Errorf("invalid argument: %s", version)
		}
	}, nil
}

// hexStringLittleEndianVariant returns the hexadecimal representation of the hash in little-endian format.
// MurmurHash3, developed by Austin Appleby, is sensitive to endianness. Other languages like Python, Ruby,
// and Java (using Guava) return a hexadecimal string in the little-endian variant. This function does the same.
func hexStringLittleEndianVariant(target string, version string) (string, error) {
	switch version {
	case v32Hex:
		h := murmur3.Sum32([]byte(target))
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, h)
		return hex.EncodeToString(b), nil
	case v128Hex:
		h1, h2 := murmur3.Sum128([]byte(target))
		b := make([]byte, 16)
		binary.LittleEndian.PutUint64(b[:8], h1)
		binary.LittleEndian.PutUint64(b[8:], h2)
		return hex.EncodeToString(b), nil
	default:
		return "", fmt.Errorf("invalid argument: %s", version)
	}
}
