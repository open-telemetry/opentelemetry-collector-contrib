// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	errDecodeID        = errors.New("could not decode ID")
	errIDInvalidLength = fmt.Errorf("%w: %w", errDecodeID, errors.New("invalid length"))
	errIDHexDecode     = fmt.Errorf("%w: %w", errDecodeID, errors.New("invalid hex"))
)

type idByteArray interface {
	pcommon.SpanID | pcommon.TraceID | pprofile.ProfileID
}

// newIDExprFunc builds an expression function that accepts either a byte slice
// of the target length or a hex string twice that size.
// If the target is a literal getter, the ID is pre-computed once for optimal performance.
func newIDExprFunc[K any, R idByteArray](funcName string, target ottl.ByteSliceLikeGetter[K]) ottl.ExprFunc[K] {
	var zero R
	idLen := len(zero)
	idHexLen := idLen * 2

	// Check if target is a literal getter, just grab the raw bytes if so
	if b, ok := ottl.GetLiteralValue(target); ok {
		id, err := bytesToID[R](funcName, b, idLen, idHexLen)
		if err != nil {
			return func(_ context.Context, _ K) (any, error) {
				return nil, err
			}
		}
		return func(_ context.Context, _ K) (any, error) {
			return id, nil
		}
	}

	// Dynamic path: evaluate on every call
	return func(ctx context.Context, tCtx K) (any, error) {
		b, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return bytesToID[R](funcName, b, idLen, idHexLen)
	}
}

// bytesToID converts a byte slice to an ID of the specified type.
// It accepts either raw bytes of length idLen or hex-encoded bytes of length idHexLen.
func bytesToID[R idByteArray](funcName string, b []byte, idLen, idHexLen int) (R, error) {
	var id R
	switch len(b) {
	case idLen:
		copyToFixedLenID(&id, b)
		return id, nil
	case idHexLen:
		decoded := make([]byte, idLen)
		if _, err := hex.Decode(decoded, b); err != nil {
			return id, fmt.Errorf("%s: %w: %w", funcName, errIDHexDecode, err)
		}
		copyToFixedLenID(&id, decoded)
		return id, nil
	default:
		return id, fmt.Errorf("%s: %w: expected %d or %d bytes, got %d", funcName, errIDInvalidLength, idLen, idHexLen, len(b))
	}
}

// copyToFixedLenID copies the bytes from the source slice to the destination fixed length array.
func copyToFixedLenID[R idByteArray](dst *R, src []byte) {
	for i := 0; i < len(src); i++ {
		(*dst)[i] = src[i]
	}
}
