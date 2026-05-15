// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logparsingfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logparsingfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type parseELFArguments struct {
	Target ottl.StringGetter[*ottllog.TransformContext]
}

// NewParseELFFactory returns an OTTL factory for the parse_elf function.
func NewParseELFFactory() ottl.Factory[*ottllog.TransformContext] {
	return ottl.NewFactory("parse_elf", &parseELFArguments{}, createParseELFFunction)
}

func createParseELFFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottllog.TransformContext], error) {
	args, ok := oArgs.(*parseELFArguments)
	if !ok {
		return nil, errors.New("parseELFFactory args must be of type *parseELFArguments")
	}
	return parseELF(args.Target), nil
}

func parseELF(target ottl.StringGetter[*ottllog.TransformContext]) ottl.ExprFunc[*ottllog.TransformContext] {
	return func(ctx context.Context, tCtx *ottllog.TransformContext) (any, error) {
		source, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if source == "" {
			return nil, errors.New("cannot parse empty ELF message")
		}
		return parseELFMessage(source)
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
func parseELFMessage(input string) (pcommon.Map, error) {
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
		entries = append(entries, elfDataEntry{
			fields: currentFields,
			values: parseELFDataLine(line),
		})
	}

	if meta.version == "" {
		return pcommon.Map{}, errors.New("invalid ELF: missing #Version directive")
	}

	return buildELFResult(meta, lastFields, entries)
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
func parseELFDataLine(line string) []string {
	var values []string
	i, n := 0, len(line)
	for i < n {
		for i < n && line[i] == ' ' {
			i++
		}
		if i >= n {
			break
		}
		if line[i] == '"' {
			i++
			start := i
			for i < n && line[i] != '"' {
				i++
			}
			values = append(values, line[start:i])
			if i < n {
				i++ // skip closing '"'
			}
		} else {
			start := i
			for i < n && line[i] != ' ' {
				i++
			}
			values = append(values, line[start:i])
		}
	}
	return values
}

func buildELFResult(meta elfMeta, lastFields []string, entries []elfDataEntry) (pcommon.Map, error) {
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
				m.PutStr(field, "-")
			}
		}
	}

	return result, nil
}
