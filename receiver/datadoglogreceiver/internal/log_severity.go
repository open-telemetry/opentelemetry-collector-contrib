// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver/internal"

import "go.opentelemetry.io/collector/pdata/plog"

// tokenLengths defines valid tokens for different lengths.
var tokenLengths = map[int][]string{
	1: {"E", "W", "I", "D", "T"},
	3: {"ERR", "WRN", "INF", "DBG", "TRC"},
	4: {"WARN", "INFO"},
	5: {"ERROR", "DEBUG", "TRACE"},
}

// tokenToSeverity maps various log level tokens to their corresponding severity levels.
// It includes common abbreviations and full names for different log levels.
var tokenToSeverity = map[string]string{
	"ERROR": "ERROR",
	"ERR":   "ERROR",
	"E":     "ERROR",
	"WARN":  "WARN",
	"WRN":   "WARN",
	"W":     "WARN",
	"INFO":  "INFO",
	"INF":   "INFO",
	"I":     "INFO",
	"DEBUG": "DEBUG",
	"DBG":   "DEBUG",
	"D":     "DEBUG",
	"TRACE": "TRACE",
	"TRC":   "TRACE",
	"T":     "TRACE",
}

// extractSeverity attempts to identify and extract a severity level from the input byte slice
func extractSeverity(input []byte) string {
	inputLen := len(input)
	// Iterate through each character in the input
	for i := 0; i < inputLen; i++ {
		// Skip non-letter characters
		if !isLetter(input[i]) {
			continue
		}

		// Try matching tokens of lengths 1 to 5
		for n := 1; n <= 5 && i+n <= inputLen; n++ {
			substr := input[i : i+n]
			tokens := tokenLengths[n]
			for _, token := range tokens {
				// Compare the substring with the token (case-insensitive)
				if equalFoldBytes(substr, []byte(token)) {
					// Check if the token is at word boundaries
					prevIsLetterOrDigit := i > 0 && isLetterOrDigit(input[i-1])
					nextIsLetterOrDigit := i+n < inputLen && isLetterOrDigit(input[i+n])
					if prevIsLetterOrDigit || nextIsLetterOrDigit {
						continue // Not a word boundary, skip this match
					}

					// Special handling for single-letter tokens
					if len(token) == 1 {
						if i+n < inputLen {
							nextChar := input[i+n]
							// Ensure the next character is a separator or space
							if !isSeparator(nextChar) && !isSpace(nextChar) {
								continue // Do not accept the match
							}
						} else {
							continue // Do not accept single-letter tokens at the end
						}
					}

					// Return the corresponding severity level
					return tokenToSeverity[token]
				}
			}
		}
	}

	// If no valid severity token is found, return "UNKNOWN"
	return "UNKNOWN"
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isLetterOrDigit(b byte) bool {
	return isLetter(b) || (b >= '0' && b <= '9')
}

func isSeparator(b byte) bool {
	switch b {
	case ':', '=', ',', ';', '.', '!', '?', '"', '\'', '[', ']', '{', '}', '(', ')', '<', '>', '/', '\\', '|', '-', '_':
		return true
	default:
		return false
	}
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func equalFoldBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]

		// Convert to lowercase if uppercase letter
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}

		if ca != cb {
			return false
		}
	}
	return true
}

func mapOtelSeverity(severity string) (plog.SeverityNumber, string) {
	switch severity {
	case "ERROR":
		return plog.SeverityNumberError, "ERROR"
	case "WARN":
		return plog.SeverityNumberWarn, "WARN"
	case "INFO":
		return plog.SeverityNumberInfo, "INFO"
	case "DEBUG", "TRACE":
		return plog.SeverityNumberDebug, "DEBUG"
	case "UNKNOWN":
		return plog.SeverityNumberUnspecified, "UNSPECIFIED"
	default:
		return plog.SeverityNumberUnspecified, "UNSPECIFIED"
	}
}
