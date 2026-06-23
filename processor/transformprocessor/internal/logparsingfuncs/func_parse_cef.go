// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logparsingfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logparsingfuncs"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type parseCEFArguments struct {
	Target ottl.StringGetter[*ottllog.TransformContext]
}

func NewParseCEFFactory() ottl.Factory[*ottllog.TransformContext] {
	return ottl.NewFactory("ParseCEF", &parseCEFArguments{}, createParseCEFFunction)
}

func createParseCEFFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottllog.TransformContext], error) {
	args, ok := oArgs.(*parseCEFArguments)
	if !ok {
		return nil, errors.New("parseCEFFactory args must be of type *parseCEFArguments")
	}

	return parseCEF(args.Target), nil
}

func parseCEF(target ottl.StringGetter[*ottllog.TransformContext]) ottl.ExprFunc[*ottllog.TransformContext] {
	return func(ctx context.Context, tCtx *ottllog.TransformContext) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if source == "" {
			return nil, errors.New("cannot parse empty CEF message")
		}

		return parseCEFMessage(source)
	}
}

type cefHeader struct {
	version            string
	deviceVendor       string
	deviceProduct      string
	deviceVersion      string
	deviceEventClassID string
	name               string
	severity           string
}

// cefExtensionKeyRegex finds extension key=value boundaries. Per the CEF spec, key
// names are alphanumeric (plus underscore) and are separated from preceding
// key=value pairs by a single space.
var cefExtensionKeyRegex = regexp.MustCompile(`(?:^| )([A-Za-z][A-Za-z0-9_]*)=`)

func parseCEFMessage(message string) (pcommon.Map, error) {
	cefStart := strings.Index(message, "CEF:")
	if cefStart == -1 {
		return pcommon.Map{}, errors.New("invalid CEF message: 'CEF:' not found")
	}

	cefMessage := message[cefStart:]

	fields, err := splitCEFHeader(cefMessage)
	if err != nil {
		return pcommon.Map{}, err
	}

	versionField := fields[0]
	if !strings.HasPrefix(versionField, "CEF:") {
		return pcommon.Map{}, fmt.Errorf("invalid CEF message: must start with 'CEF:', got %q", versionField)
	}

	version := strings.TrimPrefix(versionField, "CEF:")
	if version == "" {
		return pcommon.Map{}, errors.New("invalid CEF message: missing version")
	}

	header := cefHeader{
		version:            version,
		deviceVendor:       unescapeCEFHeader(fields[1]),
		deviceProduct:      unescapeCEFHeader(fields[2]),
		deviceVersion:      unescapeCEFHeader(fields[3]),
		deviceEventClassID: unescapeCEFHeader(fields[4]),
		name:               unescapeCEFHeader(fields[5]),
		severity:           unescapeCEFHeader(fields[6]),
	}

	var extensions map[string]any
	if len(fields) == 8 && fields[7] != "" {
		extensions = parseCEFExtensions(fields[7])
	}

	return buildCEFResult(header, extensions)
}

// splitCEFHeader splits a CEF message on unescaped pipes. The first seven
// fields are the prefix (CEF:Version) and the six header fields. Everything
// after the seventh pipe is the extension, returned as the eighth field if
// present.
func splitCEFHeader(message string) ([]string, error) {
	const headerFieldCount = 7

	fields := make([]string, 0, headerFieldCount+1)
	var current strings.Builder

	for i := 0; i < len(message); i++ {
		c := message[i]
		if c == '\\' && i+1 < len(message) {
			next := message[i+1]
			if next == '|' || next == '\\' {
				current.WriteByte(c)
				current.WriteByte(next)
				i++
				continue
			}
		}
		if c == '|' {
			fields = append(fields, current.String())
			current.Reset()
			if len(fields) == headerFieldCount {
				fields = append(fields, message[i+1:])
				return fields, nil
			}
			continue
		}
		current.WriteByte(c)
	}
	fields = append(fields, current.String())

	if len(fields) < headerFieldCount {
		return nil, fmt.Errorf("invalid CEF header: expected at least %d pipe-delimited fields (CEF:Version, Device Vendor, Device Product, Device Version, Device Event Class ID, Name, Severity), got %d", headerFieldCount, len(fields))
	}
	return fields, nil
}

// unescapeCEFHeader unescapes the two characters that may be escaped inside a
// CEF header field: pipe and backslash.
func unescapeCEFHeader(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '|' || next == '\\' {
				b.WriteByte(next)
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// unescapeCEFValue unescapes the four sequences that may appear inside a CEF
// extension value: backslash, equals, newline, and carriage return.
func unescapeCEFValue(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			case '=':
				b.WriteByte('=')
				i++
				continue
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// parseCEFExtensions parses the extension portion of a CEF message into a map
// of key/value pairs. Values may contain spaces; the parser uses the position
// of the next `key=` token as the end of the current value.
func parseCEFExtensions(extension string) map[string]any {
	result := make(map[string]any)
	matches := cefExtensionKeyRegex.FindAllStringSubmatchIndex(extension, -1)
	if len(matches) == 0 {
		return result
	}

	for i, m := range matches {
		key := extension[m[2]:m[3]]
		valueStart := m[1]
		valueEnd := len(extension)
		if i+1 < len(matches) {
			valueEnd = matches[i+1][0]
		}
		value := strings.TrimRight(extension[valueStart:valueEnd], " ")
		result[key] = unescapeCEFValue(value)
	}
	return result
}

func buildCEFResult(header cefHeader, extensions map[string]any) (pcommon.Map, error) {
	result := pcommon.NewMap()

	result.PutStr("cef.version", header.version)
	result.PutStr("cef.device_vendor", header.deviceVendor)
	result.PutStr("cef.device_product", header.deviceProduct)
	result.PutStr("cef.device_version", header.deviceVersion)
	result.PutStr("cef.device_event_class_id", header.deviceEventClassID)
	result.PutStr("cef.name", header.name)
	result.PutStr("cef.severity", header.severity)

	extensionsMap := result.PutEmptyMap("cef.extensions")
	if extensions != nil {
		if err := extensionsMap.FromRaw(extensions); err != nil {
			return pcommon.Map{}, fmt.Errorf("failed to convert extensions: %w", err)
		}
	}

	return result, nil
}
