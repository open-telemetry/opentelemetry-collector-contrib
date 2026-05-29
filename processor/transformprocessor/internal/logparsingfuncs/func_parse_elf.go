// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logparsingfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logparsingfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type parseELFArguments struct {
	Target ottl.StringGetter[*ottllog.TransformContext]
}

// NewParseELFFactory returns an OTTL factory for the parse_elf function.
func NewParseELFFactory() ottl.Factory[*ottllog.TransformContext] {
	return ottl.NewFactory("ParseELF", &parseELFArguments{}, createParseELFFunction)
}

func createParseELFFunction(fCtx ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottllog.TransformContext], error) {
	args, ok := oArgs.(*parseELFArguments)
	if !ok {
		return nil, errors.New("parseELFFactory args must be of type *parseELFArguments")
	}
	return parseELF(args.Target, fCtx.Set.Logger), nil
}

func parseELF(target ottl.StringGetter[*ottllog.TransformContext], logger *zap.Logger) ottl.ExprFunc[*ottllog.TransformContext] {
	return func(ctx context.Context, tCtx *ottllog.TransformContext) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if source == "" {
			return nil, errors.New("cannot parse empty ELF message")
		}
		return parseELFMessage(source, logger)
	}
}

// elfMeta holds the directive-level metadata from ELF header lines.
type elfMeta struct {
	version   string
	software  string
	date      string
	startDate string
	endDate   string
	remark    string
}

// elfDataEntry holds one parsed data row together with the field names that were
// active when the row was encountered (multiple #Fields directives are allowed).
type elfDataEntry struct {
	fields []string
	values []string
}

// parseELFMessage parses a W3C Extended Log Format (ELF) text block and returns a
// pcommon.Map with the following keys:
//
//   - version    – value of #Version (required; returns error if absent)
//   - software   – value of #Software (omitted if not present)
//   - date       – value of #Date (omitted if not present)
//   - start_date – value of #Start-Date (omitted if not present)
//   - end_date   – value of #End-Date (omitted if not present)
//   - remark     – value of #Remark (omitted if not present)
//   - fields     – string slice from the last #Fields directive seen
//   - entries    – slice of maps, one per data line, keyed by field name
//
// Multiple #Fields directives are supported; each applies to subsequent data lines.
func parseELFMessage(input string, logger *zap.Logger) (pcommon.Map, error) {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	lines := strings.Split(input, "\n")

	meta := elfMeta{}
	var currentFields []string
	var lastFields []string
	var entries []elfDataEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			key, value, err := parseELFDirective(line)
			if err != nil {
				// A malformed #Fields directive would silently poison subsequent data
				// lines, so treat it as a hard error.
				if strings.HasPrefix(strings.TrimSpace(line[1:]), "Fields") {
					return pcommon.Map{}, fmt.Errorf("invalid ELF: malformed #Fields directive %q: %w", line, err)
				}
				// Other unrecognised directives (e.g. bare #Remark) are skipped.
				continue
			}
			switch strings.ToLower(key) {
			case "version":
				meta.version = value
			case "fields":
				currentFields = strings.Fields(value)
				lastFields = currentFields
			case "software":
				meta.software = value
			case "date":
				meta.date = value
			case "start-date":
				meta.startDate = value
			case "end-date":
				meta.endDate = value
			case "remark":
				meta.remark = value
			}
			continue
		}

		// Data line.
		if len(currentFields) == 0 {
			return pcommon.Map{}, errors.New("invalid ELF: data entry found before #Fields directive")
		}
		values, err := parseELFDataLine(line)
		if err != nil {
			return pcommon.Map{}, fmt.Errorf("invalid ELF: %w", err)
		}
		entries = append(entries, elfDataEntry{
			fields: currentFields,
			values: values,
		})
	}

	if meta.version == "" {
		return pcommon.Map{}, errors.New("invalid ELF: missing #Version directive")
	}

	return buildELFResult(meta, lastFields, entries, logger), nil
}

// parseELFDirective parses a directive line of the form "#Key: value" and returns
// the key and trimmed value. Returns an error for lines without a colon separator.
func parseELFDirective(line string) (string, string, error) {
	body := line[1:] // strip leading '#'
	idx := strings.Index(body, ":")
	if idx == -1 {
		return "", "", fmt.Errorf("directive %q has no colon separator", line)
	}
	return strings.TrimSpace(body[:idx]), strings.TrimSpace(body[idx+1:]), nil
}

// parseELFDataLine splits a single ELF data line into tokens, honouring double-quoted
// strings as used by real-world ELF producers (e.g. Microsoft IIS).
// Whitespace (space or tab) separates tokens; embedded double-quotes inside a quoted
// string are represented by "" per the W3C ELF spec §2.
// Returns an error for unterminated quoted values.
func parseELFDataLine(line string) ([]string, error) {
	runes := []rune(line)
	var values []string
	i, n := 0, len(runes)
	for i < n {
		// skip leading whitespace
		for i < n && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= n {
			break
		}
		if runes[i] == '"' {
			i++ // skip opening '"'
			var sb strings.Builder
			closed := false
			for i < n {
				if runes[i] == '"' {
					if i+1 < n && runes[i+1] == '"' {
						// escaped double-quote: "" → "
						sb.WriteRune('"')
						i += 2
					} else {
						i++ // skip closing '"'
						closed = true
						break
					}
				} else {
					sb.WriteRune(runes[i])
					i++
				}
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted value in data line: %q", line)
			}
			values = append(values, sb.String())
		} else {
			start := i
			for i < n && !unicode.IsSpace(runes[i]) {
				i++
			}
			values = append(values, string(runes[start:i]))
		}
	}
	return values, nil
}

func buildELFResult(meta elfMeta, lastFields []string, entries []elfDataEntry, logger *zap.Logger) pcommon.Map {
	result := pcommon.NewMap()

	result.PutStr("version", meta.version)
	if meta.software != "" {
		result.PutStr("software", meta.software)
	}
	if meta.date != "" {
		result.PutStr("date", meta.date)
	}
	if meta.startDate != "" {
		result.PutStr("start_date", meta.startDate)
	}
	if meta.endDate != "" {
		result.PutStr("end_date", meta.endDate)
	}
	if meta.remark != "" {
		result.PutStr("remark", meta.remark)
	}

	fieldsSlice := result.PutEmptySlice("fields")
	for _, f := range lastFields {
		fieldsSlice.AppendEmpty().SetStr(f)
	}

	entriesSlice := result.PutEmptySlice("entries")
	for _, e := range entries {
		m := entriesSlice.AppendEmpty().SetEmptyMap()
		for i, field := range e.fields {
			if i < len(e.values) {
				m.PutStr(field, e.values[i])
			} else {
				logger.Warn("ELF data line has fewer values than fields; substituting '-'",
					zap.String("field", field),
					zap.Int("field_count", len(e.fields)),
					zap.Int("value_count", len(e.values)),
				)
				m.PutStr(field, "-")
			}
		}
	}

	return result
}
