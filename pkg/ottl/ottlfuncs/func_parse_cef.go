// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	cefPrefix       = "CEF:"
	cefHeaderFields = 7
)

type ParseCEFArguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewParseCEFFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseCEF", &ParseCEFArguments[K]{}, createParseCEFFunction[K])
}

func createParseCEFFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseCEFArguments[K])
	if !ok {
		return nil, errors.New("ParseCEFFactory args must be of type *ParseCEFArguments[K]")
	}

	return parseCEF[K](args.Target), nil
}

func parseCEF[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		cefMessage, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("error getting value for target in ParseCEF: %w", err)
		}

		if cefMessage == "" {
			return nil, errors.New("cannot parse CEF from empty target")
		}

		if !strings.HasPrefix(cefMessage, cefPrefix) {
			return nil, fmt.Errorf("CEF message must start with %q", cefPrefix)
		}

		// Remove CEF: prefix
		message := cefMessage[len(cefPrefix):]

		// Parse header fields (first 7 pipe-separated fields)
		headerParts, extensionPart, err := parseCEFHeader(message)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CEF header: %w", err)
		}

		// Build result pMap directly
		pMap := pcommon.NewMap()
		pMap.PutStr("version", unescapeCEFValue(headerParts[0]))
		pMap.PutStr("deviceVendor", unescapeCEFValue(headerParts[1]))
		pMap.PutStr("deviceProduct", unescapeCEFValue(headerParts[2]))
		pMap.PutStr("deviceVersion", unescapeCEFValue(headerParts[3]))
		pMap.PutStr("deviceEventClassId", unescapeCEFValue(headerParts[4]))
		pMap.PutStr("name", unescapeCEFValue(headerParts[5]))
		pMap.PutStr("severity", unescapeCEFValue(headerParts[6]))

		// Parse extensions directly into pMap
		extensionsMap := pMap.PutEmptyMap("extensions")
		if extensionPart != "" {
			parseCEFExtensions(extensionPart, extensionsMap)
		}

		return pMap, nil
	}
}

// parseCEFHeader splits the header into 7 pipe-separated fields and returns the extension part
func parseCEFHeader(message string) ([]string, string, error) {
	// Find the positions of unescaped pipes for the first 7 fields
	headerParts := make([]string, 0, cefHeaderFields)
	current := ""
	escaped := false
	pipeCount := 0

	for i, char := range message {
		if escaped {
			current += string(char)
			escaped = false
			continue
		}

		if char == '\\' {
			current += string(char)
			escaped = true
			continue
		}

		if char == '|' && pipeCount < cefHeaderFields-1 {
			headerParts = append(headerParts, current)
			current = ""
			pipeCount++
			continue
		}

		// If we've found all header pipes, everything else is extension
		if pipeCount == cefHeaderFields-1 && char == '|' {
			headerParts = append(headerParts, current)
			// Return the rest as extension part
			extensionPart := ""
			if i+1 < len(message) {
				extensionPart = message[i+1:]
			}
			return headerParts, extensionPart, nil
		}

		current += string(char)
	}

	// If we reach here, we need to add the last field
	if pipeCount == cefHeaderFields-1 {
		headerParts = append(headerParts, current)
		return headerParts, "", nil
	}

	return nil, "", fmt.Errorf("CEF message must have exactly %d pipe-separated header fields, found %d", cefHeaderFields, pipeCount+1)
}

// parseCEFExtensions parses the extension part directly into a pcommon.Map
func parseCEFExtensions(extensionPart string, extensionsMap pcommon.Map) {
	if strings.TrimSpace(extensionPart) == "" {
		return
	}

	i := 0
	for i < len(extensionPart) {
		i = skipSpaces(extensionPart, i)
		if i >= len(extensionPart) {
			break
		}

		key, keyEnd, found := findKey(extensionPart, i)
		if !found {
			break
		}

		i = keyEnd + 1 // Skip the =
		value, valueEnd := findValue(extensionPart, i)
		i = valueEnd

		extensionsMap.PutStr(unescapeCEFValue(key), unescapeCEFValue(value))
	}
}

// skipSpaces advances index past any whitespace characters
func skipSpaces(s string, start int) int {
	for start < len(s) && s[start] == ' ' {
		start++
	}
	return start
}

// findKey locates a key (everything up to first unescaped =)
func findKey(s string, start int) (key string, end int, found bool) {
	escaped := false
	for i := start; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		if s[i] == '\\' {
			escaped = true
			continue
		}
		if s[i] == '=' {
			return s[start:i], i, true
		}
	}
	return "", 0, false
}

// findValue locates a value using lookahead for next key=value pattern
func findValue(s string, start int) (value string, end int) {
	if start >= len(s) {
		return "", start
	}

	// Find value by looking ahead for next key=value pattern
	escaped := false
	for i := start; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		if s[i] == '\\' {
			escaped = true
			continue
		}
		if s[i] == ' ' && looksLikeNextKeyValue(s, i+1) {
			return s[start:i], i
		}
	}
	// No next key=value found, take rest of string
	return s[start:], len(s)
}

// looksLikeNextKeyValue checks if the text at position looks like "key="
func looksLikeNextKeyValue(s string, pos int) bool {
	// Skip spaces
	pos = skipSpaces(s, pos)
	if pos >= len(s) {
		return false
	}

	// Look for non-space characters followed by =
	for pos < len(s) && s[pos] != ' ' {
		if s[pos] == '=' {
			return true
		}
		pos++
	}
	return false
}

// unescapeCEFValue removes CEF escape sequences from a value
func unescapeCEFValue(value string) string {
	// Replace core CEF escape sequences only
	replacements := map[string]string{
		`\|`: "|",
		`\=`: "=",
		`\\`: "\\",
	}

	result := value
	for escaped, unescaped := range replacements {
		result = strings.ReplaceAll(result, escaped, unescaped)
	}

	return result
}
