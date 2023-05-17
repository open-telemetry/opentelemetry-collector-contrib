// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"fmt"
	"strconv"
	"strings"
)

// errTraceStateSyntax is returned for a variety of syntax errors.
var errTraceStateSyntax = fmt.Errorf("otel tracestate: %w", strconv.ErrSyntax)

// anyTraceStateParser describes how to instance types recognize
// specific fields.
type anyTraceStateParser[Instance any] interface {
	parseField(instance *Instance, key, input string) error
}

// anyTraceStateSyntax describes a variable key/value syntax.
type anyTraceStateSyntax[Instance any, Parser anyTraceStateParser[Instance]] struct {
	separator  byte
	equality   byte
	allowPunct string
}

// serializeBase adds the base fields to the output.
func (syntax anyTraceStateSyntax[Instance, Parser]) serializeBase(base *baseTraceState, sb *strings.Builder) {
	for _, field := range base.fields {
		syntax.separate(sb)
		_, _ = sb.WriteString(field)
	}
}

// separate adds a separator to the output.
func (syntax anyTraceStateSyntax[Instance, Parser]) separate(sb *strings.Builder) {
	if sb.Len() != 0 {
		_ = sb.WriteByte(syntax.separator)
	}
}

// parse uses variable syntax to parse the input string into key/value fields.
func (syntax anyTraceStateSyntax[Instance, Parser]) parse(input string) (Instance, error) {
	var parser Parser
	var invalid Instance
	var instance Instance

	if len(input) == 0 {
		return invalid, nil
	}

	for len(input) > 0 {
		eqPos := 0
		for ; eqPos < len(input); eqPos++ {
			if eqPos == 0 {
				if isLCAlpha(input[eqPos]) {
					continue
				}
			} else if isLCAlphaNum(input[eqPos]) {
				continue
			}
			break
		}
		if eqPos == 0 || eqPos == len(input) || input[eqPos] != syntax.equality {
			return invalid, errTraceStateSyntax
		}

		key := input[0:eqPos]
		tail := input[eqPos+1:]

		sepPos := 0

		for ; sepPos < len(tail); sepPos++ {
			if syntax.isValueByte(tail[sepPos]) {
				continue
			}
			break
		}

		if err := parser.parseField(&instance, key, input[0:sepPos+eqPos+1]); err != nil {
			return invalid, err
		}

		if sepPos < len(tail) && tail[sepPos] != syntax.separator {
			return invalid, errTraceStateSyntax
		}

		if sepPos == len(tail) {
			break
		}

		input = tail[sepPos+1:]

		// test for a trailing ;
		if input == "" {
			return invalid, errTraceStateSyntax
		}
	}
	return instance, nil
}

// isValueByte determines whether the byte is valid as part of a
// tracestate value.  This is based on the syntax, since the W3C syntax
// allows the OTel separator and equality symbol to appear in values.
func (syntax anyTraceStateSyntax[Instance, Parser]) isValueByte(r byte) bool {
	if isLCAlphaNum(r) {
		return true
	}
	if isUCAlpha(r) {
		return true
	}
	return strings.ContainsRune(syntax.allowPunct, rune(r))
}

// baseTraceState encodes not-specified fields as a list.  They will
// be re-encoded when serialized.
type baseTraceState struct {
	fields []string
}

// baseTraceStateParser parses not-specified fields into a list.
type baseTraceStateParser struct{}

// parseField adds to the list of not-specified fields.
func (bp baseTraceStateParser) parseField(instance *baseTraceState, _, input string) error {
	instance.fields = append(instance.fields, input)
	return nil
}

// isLCAlphaNum returns true for a-z, 0-9
func isLCAlphaNum(r byte) bool {
	if isLCAlpha(r) {
		return true
	}
	return r >= '0' && r <= '9'
}

// isLCAlphaNum returns true for a-z
func isLCAlpha(r byte) bool {
	return r >= 'a' && r <= 'z'
}

// isLCAlphaNum returns true for A-Z
func isUCAlpha(r byte) bool {
	return r >= 'A' && r <= 'Z'
}

// stripKey removes a fixed prefix from an formatted string.
func stripKey(key, input string) (string, error) {
	if len(input) < len(key)+1 {
		return "", errTraceStateSyntax
	}
	return input[len(key)+1:], nil
}
