// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/plog"
)

func createFormatter(protocol string, octetCounting bool) formatter {
	if protocol == protocolRFC5424Str {
		return newRFC5424Formatter(octetCounting)
	}
	return newRFC3164Formatter()
}

type formatter interface {
	format(plog.LogRecord) string
}

// getAttributeValueOrDefault returns the value of the requested log record's attribute as a string.
// If the attribute was not found, it returns the provided default value.
func getAttributeValueOrDefault(logRecord plog.LogRecord, attributeName, defaultValue string) string {
	value := defaultValue
	if attributeValue, found := logRecord.Attributes().Get(attributeName); found {
		value = attributeValue.AsString()
	}
	return sanitizeSyslogField(value)
}

func sanitizeSyslogField(s string) string {
	// Strip all control characters except TAB. Replace with space.
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7F {
			return ' '
		}
		return r
	}, s)
}

func sanitizeHeaderField(s string, maxLength int, fallback string) string {
	var sb strings.Builder
	for _, r := range s {
		if r >= 0x21 && r <= 0x7e {
			sb.WriteRune(r)
		}
	}
	res := sb.String()
	if res == "" {
		return fallback
	}
	if maxLength > 0 && len(res) > maxLength {
		res = res[:maxLength]
	}
	return res
}
