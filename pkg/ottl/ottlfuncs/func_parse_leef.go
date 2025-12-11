// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ParseLEEFArguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewParseLEEFFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseLEEF", &ParseLEEFArguments[K]{}, createParseLEEFFunction[K])
}

func createParseLEEFFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseLEEFArguments[K])
	if !ok {
		return nil, errors.New("ParseLEEFFactory args must be of type *ParseLEEFArguments[K]")
	}

	return parseLEEF(args.Target), nil
}

func parseLEEF[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if source == "" {
			return nil, errors.New("cannot parse empty LEEF message")
		}

		return parseLEEFMessage(source)
	}
}

func parseLEEFMessage(message string) (pcommon.Map, error) {
	// Handle optional syslog header by finding "LEEF:" in the message
	// The syslog header (if present) precedes the LEEF header and is separated by a space
	leefStart := strings.Index(message, "LEEF:")
	if leefStart == -1 {
		return pcommon.Map{}, errors.New("invalid LEEF message: 'LEEF:' not found")
	}

	// Extract just the LEEF portion (skip syslog header if present)
	leefMessage := message[leefStart:]

	// Find the first pipe to get the version field
	firstPipe := strings.Index(leefMessage, "|")
	if firstPipe == -1 {
		return pcommon.Map{}, errors.New("invalid LEEF message: missing pipe delimiter in header")
	}

	versionField := leefMessage[:firstPipe]
	version, err := parseLEEFVersion(versionField)
	if err != nil {
		return pcommon.Map{}, err
	}

	// Parse the rest based on version
	remainder := leefMessage[firstPipe+1:]

	var header leefHeader
	var attributes string

	switch version {
	case "1.0":
		header, attributes, err = parseLEEF1Header(remainder)
	case "2.0":
		header, attributes, err = parseLEEF2Header(remainder)
	default:
		return pcommon.Map{}, fmt.Errorf("unsupported LEEF version: %s", version)
	}

	if err != nil {
		return pcommon.Map{}, err
	}

	header.version = version

	// Parse attributes if present
	var parsedAttrs map[string]any
	if attributes != "" {
		parsedAttrs, err = parseLEEFAttributes(attributes, header.delimiter)
		if err != nil {
			return pcommon.Map{}, err
		}
	} else {
		parsedAttrs = make(map[string]any)
	}

	return buildLEEFResult(header, parsedAttrs)
}

type leefHeader struct {
	version        string
	vendor         string
	productName    string
	productVersion string
	eventID        string
	delimiter      string
}

func parseLEEFVersion(field string) (string, error) {
	if !strings.HasPrefix(field, "LEEF:") {
		return "", fmt.Errorf("invalid LEEF message: must start with 'LEEF:', got %q", field)
	}

	version := strings.TrimPrefix(field, "LEEF:")
	if version != "1.0" && version != "2.0" {
		return "", fmt.Errorf("unsupported LEEF version: %s (supported: 1.0, 2.0)", version)
	}

	return version, nil
}

func parseLEEF1Header(remainder string) (leefHeader, string, error) {
	// LEEF 1.0: Vendor|Product|Version|EventID|attributes
	// Attributes are tab-delimited
	parts := strings.SplitN(remainder, "|", 5)
	if len(parts) < 4 {
		return leefHeader{}, "", fmt.Errorf("invalid LEEF 1.0 header: expected at least 4 fields (vendor, product, version, eventID), got %d", len(parts))
	}

	header := leefHeader{
		vendor:         parts[0],
		productName:    parts[1],
		productVersion: parts[2],
		eventID:        parts[3],
		delimiter:      "\t", // LEEF 1.0 uses tab as default delimiter
	}

	var attributes string
	if len(parts) == 5 {
		attributes = parts[4]
	}

	return header, attributes, nil
}

func parseLEEF2Header(remainder string) (leefHeader, string, error) {
	// LEEF 2.0: Vendor|Product|Version|EventID|Delimiter|attributes
	// or: Vendor|Product|Version|EventID||attributes (empty delimiter means tab)
	parts := strings.SplitN(remainder, "|", 6)
	if len(parts) < 5 {
		return leefHeader{}, "", fmt.Errorf("invalid LEEF 2.0 header: expected at least 5 fields (vendor, product, version, eventID, delimiter), got %d", len(parts))
	}

	delimiterSpec := parts[4]
	delimiter, err := parseDelimiter(delimiterSpec)
	if err != nil {
		return leefHeader{}, "", fmt.Errorf("invalid LEEF 2.0 delimiter: %w", err)
	}

	header := leefHeader{
		vendor:         parts[0],
		productName:    parts[1],
		productVersion: parts[2],
		eventID:        parts[3],
		delimiter:      delimiter,
	}

	var attributes string
	if len(parts) == 6 {
		attributes = parts[5]
	}

	return header, attributes, nil
}

func parseDelimiter(spec string) (string, error) {
	// Empty delimiter defaults to tab
	if spec == "" {
		return "\t", nil
	}

	// Hex-encoded delimiter (e.g., "0x09" for tab, "0x5e" for caret)
	if strings.HasPrefix(spec, "0x") || strings.HasPrefix(spec, "0X") {
		hexStr := spec[2:]
		if hexStr == "" {
			return "", errors.New("empty hex value")
		}
		decoded, err := hex.DecodeString(hexStr)
		if err != nil {
			return "", fmt.Errorf("invalid hex delimiter %q: %w", spec, err)
		}
		if len(decoded) != 1 {
			return "", fmt.Errorf("hex delimiter must decode to a single byte, got %d bytes", len(decoded))
		}
		return string(decoded), nil
	}

	// Single character delimiter
	if len(spec) == 1 {
		return spec, nil
	}

	// For backwards compatibility, allow multi-character delimiters
	return spec, nil
}

func parseLEEFAttributes(attributes string, delimiter string) (map[string]any, error) {
	if attributes == "" {
		return make(map[string]any), nil
	}

	result := make(map[string]any)

	// Split by delimiter to get key=value pairs
	pairs := strings.Split(attributes, delimiter)

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split on first '=' to get key and value
		eqIndex := strings.Index(pair, "=")
		if eqIndex == -1 {
			// Key without value - skip or treat as empty value
			continue
		}

		key := pair[:eqIndex]
		value := pair[eqIndex+1:]

		if key == "" {
			continue
		}

		result[key] = value
	}

	return result, nil
}

func buildLEEFResult(header leefHeader, attributes map[string]any) (pcommon.Map, error) {
	result := pcommon.NewMap()

	result.PutStr("version", header.version)
	result.PutStr("vendor", header.vendor)
	result.PutStr("product_name", header.productName)
	result.PutStr("product_version", header.productVersion)
	result.PutStr("event_id", header.eventID)

	attrsMap := result.PutEmptyMap("attributes")
	if err := attrsMap.FromRaw(attributes); err != nil {
		return pcommon.Map{}, fmt.Errorf("failed to convert attributes: %w", err)
	}

	return result, nil
}
