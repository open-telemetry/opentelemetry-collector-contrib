// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"

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

func newParseLEEFFactory() ottl.Factory[*ottllog.TransformContext] {
	return ottl.NewFactory("parse_leef", &parseLEEFArguments{}, createParseLEEFFunction)
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
	leefStart := strings.Index(message, "LEEF:")
	if leefStart == -1 {
		return pcommon.Map{}, errors.New("invalid LEEF message: 'LEEF:' not found")
	}

	leefMessage := message[leefStart:]

	firstPipe := strings.Index(leefMessage, "|")
	if firstPipe == -1 {
		return pcommon.Map{}, errors.New("invalid LEEF message: missing pipe delimiter in header")
	}

	versionField := leefMessage[:firstPipe]
	version, err := parseLEEFVersion(versionField)
	if err != nil {
		return pcommon.Map{}, err
	}

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

	var parsedAttrs map[string]any
	if attributes != "" {
		parsedAttrs = parseLEEFAttributes(attributes, header.delimiter)
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

	if len(spec) == 1 {
		return spec, nil
	}

	return spec, nil
}

func parseLEEFAttributes(attributes, delimiter string) map[string]any {
	if attributes == "" {
		return make(map[string]any)
	}

	result := make(map[string]any)

	for pair := range strings.SplitSeq(attributes, delimiter) {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		eqIndex := strings.Index(pair, "=")
		if eqIndex == -1 {
			continue
		}

		key := pair[:eqIndex]
		value := pair[eqIndex+1:]

		if key == "" {
			continue
		}

		result[key] = value
	}

	return result
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
