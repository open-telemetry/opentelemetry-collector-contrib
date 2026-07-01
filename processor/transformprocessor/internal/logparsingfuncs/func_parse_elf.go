// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logparsingfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logparsingfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type parseELFArguments struct {
	Target ottl.StringGetter[*ottllog.TransformContext]
}

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

// nextLine returns the next non-empty trimmed line from s starting at offset,
// along with the new offset after that line's ending. Returns ok=false when
// the end of s is reached. Handles \r\n, \r, and \n line endings without
// allocating a new string.
func nextLine(s string, offset int) (line string, next int, ok bool) {
	n := len(s)
	for offset < n {
		end := offset
		for end < n && s[end] != '\n' && s[end] != '\r' {
			end++
		}
		raw := strings.TrimSpace(s[offset:end])
		if end < n && s[end] == '\r' {
			end++
		}
		if end < n && s[end] == '\n' {
			end++
		}
		offset = end
		if raw != "" {
			return raw, offset, true
		}
	}
	return "", offset, false
}

// parseELFMessage parses a W3C Extended Log Format (ELF) text block and returns a
// pcommon.Map with the following keys (all prefixed with "elf."):
//
//   - elf.version    – value of #Version (required; returns error if absent)
//   - elf.software   – value of #Software (omitted if not present)
//   - elf.date       – value of #Date (omitted if not present)
//   - elf.start_date – value of #Start-Date (omitted if not present)
//   - elf.end_date   – value of #End-Date (omitted if not present)
//   - elf.remark     – value of #Remark (omitted if not present)
//   - elf.fields     – string slice from the last #Fields directive seen
//   - elf.entries    – slice of maps, one per data line, keyed by "elf.<field>"
//
// Multiple #Fields directives are supported; each applies to subsequent data lines.
func parseELFMessage(input string, logger *zap.Logger) (pcommon.Map, error) {
	result := pcommon.NewMap()
	entriesSlice := result.PutEmptySlice("elf.entries")

	var hasVersion bool
	var currentFields []string
	var lastFields []string

	offset := 0
	for {
		line, next, ok := nextLine(input, offset)
		if !ok {
			break
		}
		offset = next

		if strings.HasPrefix(line, "#") {
			key, value, err := parseELFDirective(line)
			if err != nil {
				// A malformed #Fields directive would silently poison subsequent data
				// lines, so treat it as a hard error.
				trimmed := strings.TrimSpace(line[1:])
				if len(trimmed) >= 6 && strings.EqualFold(trimmed[:6], "fields") {
					return pcommon.Map{}, fmt.Errorf("invalid ELF: malformed #Fields directive %q: %w", line, err)
				}
				// Other unrecognized directives (e.g. bare #Remark) are skipped.
				continue
			}
			switch {
			case strings.EqualFold(key, "version"):
				hasVersion = true
				result.PutStr("elf.version", value)
			case strings.EqualFold(key, "fields"):
				currentFields = strings.Fields(value)
				lastFields = currentFields
			case strings.EqualFold(key, "software"):
				result.PutStr("elf.software", value)
			case strings.EqualFold(key, "date"):
				result.PutStr("elf.date", value)
			case strings.EqualFold(key, "start-date"):
				result.PutStr("elf.start_date", value)
			case strings.EqualFold(key, "end-date"):
				result.PutStr("elf.end_date", value)
			case strings.EqualFold(key, "remark"):
				result.PutStr("elf.remark", value)
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
		m := entriesSlice.AppendEmpty().SetEmptyMap()
		for i, field := range currentFields {
			if i < len(values) {
				m.PutStr("elf."+field, values[i])
			} else {
				logger.Warn("ELF data line has fewer values than fields; substituting '-'",
					zap.String("field", field),
					zap.Int("field_count", len(currentFields)),
					zap.Int("value_count", len(values)),
				)
				m.PutStr("elf."+field, "-")
			}
		}
	}

	if !hasVersion {
		return pcommon.Map{}, errors.New("invalid ELF: missing #Version directive")
	}

	fieldsSlice := result.PutEmptySlice("elf.fields")
	for _, f := range lastFields {
		fieldsSlice.AppendEmpty().SetStr(f)
	}

	return result, nil
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

// parseELFDataLine splits a single ELF data line into tokens, honoring double-quoted
// strings as used by real-world ELF producers (e.g. Microsoft IIS).
// Whitespace (space or tab) separates tokens; embedded double-quotes inside a quoted
// string are represented by "" per the W3C ELF spec §2.
// Returns an error for unterminated quoted values.
func parseELFDataLine(line string) ([]string, error) {
	var values []string
	i, n := 0, len(line)
	for i < n {
		// skip leading whitespace (space or tab)
		for i < n && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		if line[i] == '"' {
			i++ // skip opening '"'
			var sb strings.Builder
			closed := false
			for i < n {
				if line[i] != '"' {
					sb.WriteByte(line[i])
					i++
					continue
				}
				if i+1 < n && line[i+1] == '"' {
					// escaped double-quote: "" → "
					sb.WriteByte('"')
					i += 2
				} else {
					i++ // skip closing '"'
					closed = true
					break
				}
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted value in data line: %q", line)
			}
			values = append(values, sb.String())
		} else {
			start := i
			for i < n && line[i] != ' ' && line[i] != '\t' {
				i++
			}
			values = append(values, line[start:i])
		}
	}
	return values, nil
}
