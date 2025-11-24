// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	errIDInvalidLength = errors.New("invalid id length")
	errIDHexDecode     = errors.New("invalid id hex")
)

type IDByteArray interface {
	~[8]byte | ~[16]byte
}

// newIDExprFunc builds an expression function that accepts either a byte slice
// of the target length or a hex string twice that size.
func newIDExprFunc[K any, R IDByteArray](target ottl.ByteSliceLikeGetter[K]) ottl.ExprFunc[K] {
	var zero R
	idLen := len(zero)
	idHexLen := idLen * 2

	return func(ctx context.Context, tCtx K) (any, error) {
		b, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		var id R
		switch len(b) {
		case idLen:
			copyToFixedLenID(&id, b)
			return id, nil
		case idHexLen:
			decoded := make([]byte, idLen)
			if _, err := hex.Decode(decoded, b); err != nil {
				return nil, fmt.Errorf("%w: %w", errIDHexDecode, err)
			}
			copyToFixedLenID(&id, decoded)
			return id, nil
		default:
			return nil, fmt.Errorf("%w: expected %d or %d bytes, got %d", errIDInvalidLength, idLen, idHexLen, len(b))
		}
	}
}

// copyToFixedLenID copies the bytes from the source slice to the destination fixed length array.
func copyToFixedLenID[R IDByteArray](dst *R, src []byte) {
	for i := 0; i < len(src); i++ {
		(*dst)[i] = src[i]
	}
}
