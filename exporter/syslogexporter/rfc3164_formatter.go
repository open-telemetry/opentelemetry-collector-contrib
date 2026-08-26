// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/plog"
)

type rfc3164Formatter struct{}

func newRFC3164Formatter() *rfc3164Formatter {
	return &rfc3164Formatter{}
}

func (f *rfc3164Formatter) format(logRecord plog.LogRecord) string {
	priorityString := f.formatPriority(logRecord)
	timestampString := f.formatTimestamp(logRecord)
	hostnameString := f.formatHostname(logRecord)
	appnameString := f.formatAppname(logRecord)
	messageString := f.formatMessage(logRecord)
	appnameMessageDelimiter := ""
	if appnameString != "" && messageString != emptyMessage {
		appnameMessageDelimiter = " "
	}
	formatted := fmt.Sprintf("<%s>%s %s %s%s%s\n", priorityString, timestampString, hostnameString, appnameString, appnameMessageDelimiter, messageString)
	return formatted
}

func (*rfc3164Formatter) formatPriority(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, priority, strconv.Itoa(defaultPriority))
}

func (*rfc3164Formatter) formatTimestamp(logRecord plog.LogRecord) string {
	return logRecord.Timestamp().AsTime().Format("Jan _2 15:04:05")
}

func (*rfc3164Formatter) formatHostname(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, hostname, emptyValue)
}

func (*rfc3164Formatter) formatAppname(logRecord plog.LogRecord) string {
	value := getAttributeValueOrDefault(logRecord, app, "")
	if value != "" {
		value += ":"
	}
	return value
}

func (*rfc3164Formatter) formatMessage(logRecord plog.LogRecord) string {
	return getAttributeValueOrDefault(logRecord, message, emptyMessage)
}
