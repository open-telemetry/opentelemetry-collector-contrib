// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parseutils

import "fmt"

// SplitString will split the input on the delimiter and return the resulting slice while respecting quotes. Outer quotes are stripped.
// Use in place of `strings.Split` when quotes need to be respected.
// Requires `delimiter` not be an empty string
func SplitString(input, delimiter string) ([]string, error) {
	var result []string
	current := ""
	delimiterLength := len(delimiter)
	quoteChar := "" // "" means we are not in quotes

	for i := 0; i < len(input); i++ {
		if quoteChar == "" && i+delimiterLength <= len(input) && input[i:i+delimiterLength] == delimiter { // delimiter
			if current == "" { // leading || trailing delimiter; ignore
				i += delimiterLength - 1
				continue
			}
			result = append(result, current)
			current = ""
			i += delimiterLength - 1
			continue
		}

		if quoteChar == "" && (input[i] == '"' || input[i] == '\'') { // start of quote
			quoteChar = string(input[i])
			continue
		}
		if string(input[i]) == quoteChar { // end of quote
			quoteChar = ""
			continue
		}

		current += string(input[i])
	}

	if quoteChar != "" { // check for closed quotes
		return nil, fmt.Errorf("never reached the end of a quoted value")
	}
	if current != "" { // avoid adding empty value bc of a trailing delimiter
		return append(result, current), nil
	}

	return result, nil
}
