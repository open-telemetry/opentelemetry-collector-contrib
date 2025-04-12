// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/hex"
	"errors"
)

var (
	errInvalidTraceIDLength = errors.New("TraceID must be a 32 character hex string, like: 'ae87dadd90e9935a4bc9660628efd569'")
	errInvalidSpanIDLength  = errors.New("SpanID must be a 16 character hex string, like: '5828fa4960140870'")
	errInvalidTraceID       = errors.New("failed to create traceID byte array from the given traceID, make sure the traceID is a hex representation of a [16]byte, like: 'ae87dadd90e9935a4bc9660628efd569'")
	errInvalidSpanID        = errors.New("failed to create SpanID byte array from the given SpanID, make sure the SpanID is a hex representation of a [8]byte, like: '5828fa4960140870'")
)

func ValidateTraceID(traceID string) error {
	if len(traceID) != 32 {
		return errInvalidTraceIDLength
	}

	_, err := hex.DecodeString(traceID)
	if err != nil {
		return errInvalidTraceID
	}

	return nil
}

func ValidateSpanID(spanID string) error {
	if len(spanID) != 16 {
		return errInvalidSpanIDLength
	}
	_, err := hex.DecodeString(spanID)
	if err != nil {
		return errInvalidSpanID
	}

	return nil
}
