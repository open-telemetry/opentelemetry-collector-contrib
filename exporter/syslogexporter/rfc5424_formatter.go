// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type rfc5424Formatter struct {
	octetCounting bool
}

func newRFC5424Formatter(octetCounting bool) *rfc5424Formatter {
	return &rfc5424Formatter{
		octetCounting: octetCounting,
	}
}

func (f *rfc5424Formatter) format(logRecord plog.LogRecord) string {
	priorityString := f.formatPriority(logRecord)
	versionString := f.formatVersion(logRecord)
	timestampString := f.formatTimestamp(logRecord)
	hostnameString := f.formatHostname(logRecord)
	appnameString := f.formatAppname(logRecord)
	pidString := f.formatPid(logRecord)
	messageIDString := f.formatMessageID(logRecord)
	structuredData := f.formatStructuredData(logRecord)
	messageString := f.formatMessage(logRecord)
	formatted := fmt.Sprintf("<%s>%s %s %s %s %s %s %s%s\n", priorityString, versionString, timestampString, hostnameString, appnameString, pidString, messageIDString, structuredData, messageString)

	if f.octetCounting {
		formatted = fmt.Sprintf("%d %s", len(formatted), formatted)
	}

	return formatted
}

func (f *rfc5424Formatter) formatPriority(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, priority, strconv.Itoa(defaultPriority))
}

func (f *rfc5424Formatter) formatVersion(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, version, strconv.Itoa(versionRFC5424))
}

func (f *rfc5424Formatter) formatTimestamp(logRecord plog.LogRecord) string {
	return logRecord.Timestamp().AsTime().Format(time.RFC3339Nano)
}

func (f *rfc5424Formatter) formatHostname(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, hostname, emptyValue)
}

func (f *rfc5424Formatter) formatAppname(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, app, emptyValue)
}

func (f *rfc5424Formatter) formatPid(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, pid, emptyValue)
}

func (f *rfc5424Formatter) formatMessageID(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, msgID, emptyValue)
}

func (f *rfc5424Formatter) formatStructuredData(logRecord plog.LogRecord) string {
	structuredDataAttributeValue, found := logRecord.Attributes().Get(structuredData)
	if !found {
		return emptyValue
	}
	if structuredDataAttributeValue.Type() != pcommon.ValueTypeMap {
		return emptyValue
	}

	var sdBuilder strings.Builder
	for key, val := range structuredDataAttributeValue.Map().AsRaw() {
		sdElements := []string{key}
		vval, ok := val.(map[string]any)
		if !ok {
			continue
		}
		for k, v := range vval {
			vv, ok := v.(string)
			if !ok {
				continue
			}
			sdElements = append(sdElements, fmt.Sprintf("%s=\"%s\"", k, vv))
		}
		sdBuilder.WriteString(fmt.Sprint(sdElements))
	}
	return sdBuilder.String()
}

func (f *rfc5424Formatter) formatMessage(logRecord plog.LogRecord) string {
	formatted := getAttributeValueOrDefault(logRecord, message, emptyMessage)
	if len(formatted) > 0 {
		formatted = " " + formatted
	}
	return formatted
}
