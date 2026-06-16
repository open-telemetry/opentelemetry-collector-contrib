// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logparsingfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logparsingfuncs"

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type parseLEEFArguments struct {
	Target ottl.StringGetter[*ottllog.TransformContext]
}

func NewParseLEEFFactory() ottl.Factory[*ottllog.TransformContext] {
	return ottl.NewFactory("ParseLEEF", &parseLEEFArguments{}, createParseLEEFFunction)
}

func createParseLEEFFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottllog.TransformContext], error) {
	args, ok := oArgs.(*parseLEEFArguments)
	if !ok {
		return nil, errors.New("parseLEEFFactory args must be of type *parseLEEFArguments")
	}

	return parseLEEF(args.Target), nil
}

func parseLEEF(target ottl.StringGetter[*ottllog.TransformContext]) ottl.ExprFunc[*ottllog.TransformContext] {
	return func(ctx context.Context, tCtx *ottllog.TransformContext) (any, error) {
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
	// Locate the LEEF header by the first occurrence of "LEEF:" so that an
	// optional syslog prefix is tolerated. A literal "LEEF:" appearing inside a
	// syslog header (e.g. structured data) before the real header would be
	// misinterpreted; this matches the convention used by other LEEF parsers.
	leefStart := strings.Index(message, "LEEF:")
	if leefStart == -1 {
		return pcommon.Map{}, errors.New("invalid LEEF message: 'LEEF:' not found")
	}

	leefMessage := message[leefStart:]

	versionField, remainder, ok := strings.Cut(leefMessage, "|")
	if !ok {
		return pcommon.Map{}, errors.New("invalid LEEF message: missing pipe delimiter in header")
	}

	version, err := parseLEEFVersion(versionField)
	if err != nil {
		return pcommon.Map{}, err
	}

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

	parsedAttrs := parseLEEFAttributes(attributes, header.delimiter)

	return buildLEEFResult(header, parsedAttrs), nil
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
	parts := strings.SplitN(remainder, "|", 5)
	if len(parts) < 4 {
		return leefHeader{}, "", fmt.Errorf("invalid LEEF 1.0 header: expected at least 4 fields (vendor, product, version, eventID), got %d", len(parts))
	}

	header := leefHeader{
		vendor:         parts[0],
		productName:    parts[1],
		productVersion: parts[2],
		eventID:        parts[3],
		delimiter:      "\t",
	}

	var attributes string
	if len(parts) == 5 {
		attributes = parts[4]
	}

	return header, attributes, nil
}

func parseLEEF2Header(remainder string) (leefHeader, string, error) {
	parts := strings.SplitN(remainder, "|", 6)
	if len(parts) < 4 {
		return leefHeader{}, "", fmt.Errorf("invalid LEEF 2.0 header: expected at least 4 fields (vendor, product, version, eventID), got %d", len(parts))
	}

	header := leefHeader{
		vendor:         parts[0],
		productName:    parts[1],
		productVersion: parts[2],
		eventID:        parts[3],
	}

	// The LEEF 2.0 delimiter field is optional per the spec. If it is omitted
	// entirely the header ends after the eventID; default the attribute
	// delimiter to tab and leave attributes empty (mirrors LEEF 1.0).
	if len(parts) == 4 {
		header.delimiter = "\t"
		return header, "", nil
	}

	// The delimiter field may also be omitted while attributes are present. If
	// the 5th field contains '=' it is the first attribute, not a delimiter —
	// re-split with one fewer segment so any '|' characters inside the
	// attributes section are preserved.
	delimiterSpec := parts[4]
	if strings.Contains(delimiterSpec, "=") {
		header.delimiter = "\t"
		attrParts := strings.SplitN(remainder, "|", 5)
		var attributes string
		if len(attrParts) == 5 {
			attributes = attrParts[4]
		}
		return header, attributes, nil
	}

	delimiter, err := parseDelimiter(delimiterSpec)
	if err != nil {
		return leefHeader{}, "", fmt.Errorf("invalid LEEF 2.0 delimiter: %w", err)
	}
	header.delimiter = delimiter

	var attributes string
	if len(parts) == 6 {
		attributes = parts[5]
	}

	return header, attributes, nil
}

func parseDelimiter(spec string) (string, error) {
	if spec == "" {
		return "\t", nil
	}

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

	if len(spec) != 1 {
		return "", fmt.Errorf("delimiter must be a single character or 0x-prefixed hex value, got %q", spec)
	}
	return spec, nil
}

// parseLEEFAttributes splits the LEEF attribute section into key/value pairs.
// Pairs missing an '=' or with an empty key are silently skipped; on duplicate
// keys the last occurrence wins. This matches the lenient behavior described
// in the LEEF spec where producers may emit malformed pairs. Whitespace within
// keys and values is preserved verbatim: the spec defines a value as everything
// up to the delimiter, so trimming would discard data the source emitted.
func parseLEEFAttributes(attributes, delimiter string) map[string]string {
	result := make(map[string]string)
	if attributes == "" {
		return result
	}

	for pair := range strings.SplitSeq(attributes, delimiter) {
		if pair == "" {
			continue
		}

		key, value, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			continue
		}

		result[key] = value
	}

	return result
}

func buildLEEFResult(header leefHeader, attributes map[string]string) pcommon.Map {
	result := pcommon.NewMap()

	result.PutStr("leef.version", header.version)
	result.PutStr("leef.vendor", header.vendor)
	result.PutStr("leef.product.name", header.productName)
	result.PutStr("leef.product.version", header.productVersion)
	result.PutStr("leef.event.id", header.eventID)

	attrsMap := result.PutEmptyMap("leef.attributes")
	attrsMap.EnsureCapacity(len(attributes))
	for k, v := range attributes {
		attrsMap.PutStr(k, v)
	}

	return result
}
