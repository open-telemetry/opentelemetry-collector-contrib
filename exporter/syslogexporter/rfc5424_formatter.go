// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/leodido/go-syslog/v4/rfc5424"
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

func (*rfc5424Formatter) formatPriority(logRecord plog.LogRecord) string {
	priorityStr := getAttributeValueOrDefault(logRecord, priority, strconv.Itoa(defaultPriority))
	if _, err := strconv.Atoi(priorityStr); err != nil {
		return strconv.Itoa(defaultPriority)
	}
	return priorityStr
}

func (*rfc5424Formatter) formatVersion(logRecord plog.LogRecord) string {
	versionStr := getAttributeValueOrDefault(logRecord, version, strconv.Itoa(versionRFC5424))
	if _, err := strconv.Atoi(versionStr); err != nil {
		return strconv.Itoa(versionRFC5424)
	}
	return versionStr
}

func (*rfc5424Formatter) formatTimestamp(logRecord plog.LogRecord) string {
	return logRecord.Timestamp().AsTime().Format(rfc5424.RFC3339MICRO)
}

func (*rfc5424Formatter) formatHostname(logRecord plog.LogRecord) string {
	val := getAttributeValueOrDefault(logRecord, hostname, emptyValue)
	return sanitizeHeaderField(val, 255, emptyValue)
}

func (*rfc5424Formatter) formatAppname(logRecord plog.LogRecord) string {
	val := getAttributeValueOrDefault(logRecord, app, emptyValue)
	return sanitizeHeaderField(val, 48, emptyValue)
}

func (*rfc5424Formatter) formatPid(logRecord plog.LogRecord) string {
	val := getAttributeValueOrDefault(logRecord, pid, emptyValue)
	return sanitizeHeaderField(val, 128, emptyValue)
}

func (*rfc5424Formatter) formatMessageID(logRecord plog.LogRecord) string {
	val := getAttributeValueOrDefault(logRecord, msgID, emptyValue)
	return sanitizeHeaderField(val, 32, emptyValue)
}

func (*rfc5424Formatter) formatStructuredData(logRecord plog.LogRecord) string {
	structuredDataAttributeValue, found := logRecord.Attributes().Get(structuredData)
	if !found {
		return emptyValue
	}
	if structuredDataAttributeValue.Type() != pcommon.ValueTypeMap {
		return emptyValue
	}

	var sdBuilder strings.Builder
	for key, val := range structuredDataAttributeValue.Map().AsRaw() {
		cleanKey := sanitizeSDName(key, true)
		if cleanKey == "" {
			continue
		}
		sdElements := []string{cleanKey}
		vval, ok := val.(map[string]any)
		if !ok {
			continue
		}
		for k, v := range vval {
			vv, ok := v.(string)
			if !ok {
				continue
			}
			cleanK := sanitizeSDName(k, false)
			if cleanK == "" {
				continue
			}
			sdElements = append(sdElements, fmt.Sprintf("%s=%q", cleanK, sanitizeSyslogField(vv)))
		}
		fmt.Fprint(&sdBuilder, sdElements)
	}
	if sdBuilder.Len() == 0 {
		return emptyValue
	}
	return sdBuilder.String()
}

func (*rfc5424Formatter) formatMessage(logRecord plog.LogRecord) string {
	formatted := getAttributeValueOrDefault(logRecord, message, emptyMessage)
	if formatted != "" {
		formatted = " " + formatted
	}
	return formatted
}

func sanitizeSDName(s string, allowAt bool) string {
	var sb strings.Builder
	for _, r := range s {
		if r >= 0x21 && r <= 0x7e {
			if r == '=' || r == ']' || r == '"' || (!allowAt && r == '@') {
				continue
			}
			sb.WriteRune(r)
		}
	}
	res := sb.String()
	if len(res) > 32 {
		res = res[:32]
	}
	return res
}
