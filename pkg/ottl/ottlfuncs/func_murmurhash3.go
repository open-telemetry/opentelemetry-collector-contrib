// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/spaolacci/murmur3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	v32  = "32"
	v128 = "128" // default
)

type MurmurHash3Arguments[K any] struct {
	Target  ottl.StringGetter[K]
	Version ottl.Optional[string] // 32-bit or 128-bit
}

func NewMurmurHash3Factory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("MurmurHash3", &MurmurHash3Arguments[K]{}, createMurmurHash3Function[K])
}

func createMurmurHash3Function[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*MurmurHash3Arguments[K])

	if !ok {
		return nil, fmt.Errorf("MurmurHash3Factory args must be of type *MurmurHash3Arguments[K]")
	}

	version := v128
	if !args.Version.IsEmpty() {
		if (args.Version.Get() != v32) && (args.Version.Get() != v128) {
			return nil, fmt.Errorf("invalid arguments: %s. Version should be either \"32\" or \"128\"", args.Version.Get())
		}
		version = args.Version.Get()
	}

	return MurmurHash3HexString(args.Target, version)
}

// MurmurHash3HexString returns the hexadecimal representation of the hash in little-endian format.
// MurmurHash3, developed by Austin Appleby, is sensitive to endianness. Unlike some other languages like Python,
// which use little-endian for all architectures, the Go library `spaolacci/murmur3` has some open issues
// related to endianness compatibility across languages. This function ensures consistency by using
// little-endian and returns the hash value as a hexadecimal string.
func MurmurHash3HexString[K any](target ottl.StringGetter[K], version string) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch version {
		case v32:
			h := murmur3.Sum32([]byte(val))
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, h)
			return hex.EncodeToString(b), nil
		case v128:
			h1, h2 := murmur3.Sum128([]byte(val))
			b := make([]byte, 16)
			binary.LittleEndian.PutUint64(b[:8], h1)
			binary.LittleEndian.PutUint64(b[8:], h2)
			return hex.EncodeToString(b), nil
		default:
			return nil, fmt.Errorf("invalid argument: %s", version)
		}
	}, nil
}
