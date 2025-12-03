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
	errIDInvalidLength = errors.New("invalid ID length")
	errIDHexDecode     = errors.New("invalid ID hex")
)

// funcErrorType is an error type that includes the function name that caused the error.
type funcErrorType struct {
	funcName   string
	innerError error
}

func (e *funcErrorType) Error() string {
	return fmt.Sprintf("%s could not decode ID: %v", e.funcName, e.innerError)
}

func (e *funcErrorType) Unwrap() error {
	return e.innerError
}

func funcError(funcName string, err error) error {
	return &funcErrorType{
		funcName:   funcName,
		innerError: err,
	}
}

type idByteArray interface {
	pcommon.SpanID | pcommon.TraceID | pprofile.ProfileID
}

// newIDExprFunc builds an expression function that accepts either a byte slice
// of the target length or a hex string twice that size.
func newIDExprFunc[K any, R idByteArray](funcName string, target ottl.ByteSliceLikeGetter[K]) ottl.ExprFunc[K] {
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
				return nil, funcError(funcName, fmt.Errorf("%w: %w", errIDHexDecode, err))
			}
			copyToFixedLenID(&id, decoded)
			return id, nil
		default:
			return nil, funcError(funcName, fmt.Errorf("%w: expected %d or %d bytes, got %d", errIDInvalidLength, idLen, idHexLen, len(b)))
		}
	}
}

// copyToFixedLenID copies the bytes from the source slice to the destination fixed length array.
func copyToFixedLenID[R idByteArray](dst *R, src []byte) {
	for i := 0; i < len(src); i++ {
		(*dst)[i] = src[i]
	}
}
