// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"strings"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/plog"
)

func createFormatter(protocol, network string, octetCounting bool) formatter {
	// A datagram holds exactly one syslog message, so line breaks in the message cannot start a new one.
	datagram := network == string(confignet.TransportTypeUDP) || network == string(confignet.TransportTypeUnixgram)
	if protocol == protocolRFC5424Str {
		f := newRFC5424Formatter(octetCounting)
		f.rawMessage = f.rawMessage || datagram
		return f
	}
	f := newRFC3164Formatter()
	f.rawMessage = datagram
	return f
}

type formatter interface {
	format(plog.LogRecord) string
}

// delimiterReplacer replaces LF, CR and NUL with a space. Without octet counting, receivers of
// syslog over a stream socket end a message at LF or NUL (RFC 6587, section 3.4.2), so these
// characters in a field would end the current message and start a new one. CR is replaced too,
// so that CR LF becomes a single space and no bare CR is left in a header field.
var delimiterReplacer = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\x00", " ")

// replaceDelimiters returns s with LF, CR and NUL replaced by a space.
func replaceDelimiters(s string) string {
	if !strings.ContainsAny(s, "\r\n\x00") {
		return s
	}
	return delimiterReplacer.Replace(s)
}

// getAttributeValueOrDefault returns the value of the requested log record's attribute as a string,
// with LF, CR and NUL replaced by a space.
// If the attribute was not found, it returns the provided default value.
func getAttributeValueOrDefault(logRecord plog.LogRecord, attributeName, defaultValue string) string {
	return replaceDelimiters(getRawAttributeValueOrDefault(logRecord, attributeName, defaultValue))
}

// getMessageOrDefault returns the message attribute. If raw is false, LF, CR and NUL are replaced by a space.
func getMessageOrDefault(logRecord plog.LogRecord, raw bool) string {
	if raw {
		return getRawAttributeValueOrDefault(logRecord, message, emptyMessage)
	}
	return getAttributeValueOrDefault(logRecord, message, emptyMessage)
}

// getRawAttributeValueOrDefault is like getAttributeValueOrDefault, but returns the value unchanged.
func getRawAttributeValueOrDefault(logRecord plog.LogRecord, attributeName, defaultValue string) string {
	value := defaultValue
	if attributeValue, found := logRecord.Attributes().Get(attributeName); found {
		value = attributeValue.AsString()
	}
	return value
}
