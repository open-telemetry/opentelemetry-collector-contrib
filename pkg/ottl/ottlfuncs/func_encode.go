// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"unicode/utf8"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type EncodeArguments[K any] struct {
	Target      ottl.Getter[K]
	Encoding    string
	Replacement ottl.Optional[ottl.StringGetter[K]]
}

func NewEncodeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Encode", &EncodeArguments[K]{}, createEncodeFunction[K])
}

func createEncodeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*EncodeArguments[K])
	if !ok {
		return nil, errors.New("EncodeFactory args must be of type *EncodeArguments[K]")
	}

	return Encode(args.Target, args.Encoding, args.Replacement)
}

func Encode[K any](target ottl.Getter[K], encoding string, replacementCharset ottl.Optional[ottl.StringGetter[K]]) (ottl.ExprFunc[K], error) {
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

		var replacementStr string
		if !replacementCharset.IsEmpty() {
			replacementGetter := replacementCharset.Get()
			replacementStr, err = replacementGetter.Get(ctx, tCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to get replacement charset: %w", err)
			}
		} else {
			replacementStr = string(utf8.RuneError)
		}

		switch encoding {
		case "base64":
			return base64.StdEncoding.EncodeToString([]byte(stringValue)), nil
		case "base64-raw":
			return base64.RawStdEncoding.EncodeToString([]byte(stringValue)), nil
		case "base64-url":
			return base64.URLEncoding.EncodeToString([]byte(stringValue)), nil
		case "base64-raw-url":
			return base64.RawURLEncoding.EncodeToString([]byte(stringValue)), nil
		case "utf8-raw", "utf-8-raw", "nop":
			// Raw encodings are nop and should not sanitize the input.
			return stringValue, nil
		case "ascii", "us-ascii":
			// ASCII encodings cannot be handled by 'textutils.LookupEncoding' because they are overridden to UTF-8.
			// This is correct for decoding because ASCII is a subset of UTF-8, but not for encoding.
			return encodeASCII(stringValue)
		default:
			e, err := textutils.LookupEncoding(encoding)
			if err != nil {
				return nil, err
			}

			// Function returns a string. In collector, all strings are supposed to be valid UTF-8.
			// So we need to sanitize the input to ensure it is valid UTF-8.
			encodedString, err := encodeAndSanitize(e, stringValue, replacementStr)
			if err != nil {
				return nil, fmt.Errorf("could not encode: %w", err)
			}

			return encodedString, nil
		}
	}, nil
}

func encodeASCII(input string) (string, error) {
	// Both variants map to the same IANA-registered US-ASCII encoding.
	e, err := ianaindex.IANA.Encoding("us-ascii")
	if err != nil {
		return "", fmt.Errorf("failed to get ASCII encoder: %w", err)
	}

	encodedString, err := e.NewEncoder().String(input)
	if err != nil {
		return "", fmt.Errorf("could not encode: %w", err)
	}

	return encodedString, nil
}

func encodeAndSanitize(e encoding.Encoding, input, replacement string) (string, error) {
	encodedString, err := e.NewEncoder().String(input)
	// If encoding fails (unmappable characters), sanitize input first
	if err != nil {
		sanitized := replaceInvalidUTF8(input, replacement)
		encodedString, err = e.NewEncoder().String(sanitized)
		if err != nil {
			return "", err
		}
	}

	// Check if the output contains invalid UTF-8 bytes
	// (Common with legacy charsets like ISO-8859-1, Windows-1252)
	if !utf8.ValidString(encodedString) || (e == unicode.UTF8 && replacement != string(utf8.RuneError)) {
		// Replace invalid UTF-8 bytes with the replacement character
		encodedString = replaceInvalidUTF8(encodedString, replacement)
	}

	return encodedString, nil
}

func replaceInvalidUTF8(input, replacement string) string {
	var buf bytes.Buffer

	for input != "" {
		r, size := utf8.DecodeRuneInString(input)
		if r == utf8.RuneError {
			buf.WriteString(replacement)
		} else {
			buf.WriteRune(r)
		}
		input = input[size:]
	}

	return buf.String()
}
