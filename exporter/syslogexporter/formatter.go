// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"go.opentelemetry.io/collector/pdata/plog"
)

func createFormatter(protocol string) formatter {
	if protocol == protocolRFC5424Str {
		return newRFC5424Formatter()
	}
	return newRFC3164Formatter()
}

type formatter interface {
	format(plog.LogRecord) string
}

// getAttributeValueOrDefault returns the value of the requested log record's attribute as a string.
// If the attribute was not found, it returns the provided default value.
func getAttributeValueOrDefault(logRecord plog.LogRecord, attributeName string, defaultValue string) string {
	value := defaultValue
	if attributeValue, found := logRecord.Attributes().Get(attributeName); found {
		value = attributeValue.AsString()
	}
	return value
}
