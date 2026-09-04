// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package journald // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"

import (
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// priorityToSeverity maps journald PRIORITY (syslog severity) values to OTel entry.Severity.
// Journald/syslog PRIORITY: 0=emerg, 1=alert, 2=crit, 3=err, 4=warning, 5=notice, 6=info, 7=debug.
// The severity numbers follow the syslog mapping from the OTel logs data model:
// https://opentelemetry.io/docs/specs/otel/logs/data-model-appendix/#appendix-b-severitynumber-example-mappings
// See also: https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html
var priorityToSeverity = map[string]entry.Severity{
	"0": entry.Fatal,  // emerg
	"1": entry.Error3, // alert
	"2": entry.Error2, // crit
	"3": entry.Error,  // err
	"4": entry.Warn,   // warning
	"5": entry.Info2,  // notice
	"6": entry.Info,   // info
	"7": entry.Debug,  // debug
}

// priorityToSeverityText maps journald PRIORITY values to OTel severity text.
// The original syslog level name is kept as the severity text, as recommended by
// https://opentelemetry.io/docs/specs/otel/logs/data-model-appendix/#appendix-b-severitynumber-example-mappings
var priorityToSeverityText = map[string]string{
	"0": "emerg",
	"1": "alert",
	"2": "crit",
	"3": "err",
	"4": "warning",
	"5": "notice",
	"6": "info",
	"7": "debug",
}

// attributeMapping maps journald well-known field names to OTel semantic convention log attribute names.
// Journald fields: https://www.freedesktop.org/software/systemd/man/latest/systemd.journal-fields.html
// The syslog.* attribute names follow the RFC5424 syslog mapping of the OTel logs data model:
// https://opentelemetry.io/docs/specs/otel/logs/data-model-appendix/#rfc5424-syslog
// They are not (yet) part of the semantic conventions registry.
var attributeMapping = map[string]string{
	"CODE_FILE":       "code.file.path",
	"CODE_FUNC":       "code.function.name",
	"CODE_LINE":       "code.line.number",
	"SYSLOG_FACILITY": "syslog.facility.code",
	// SYSLOG_IDENTIFIER is documented as the equivalent of the RFC5424 APP-NAME,
	// which the OTel logs data model maps to syslog.identifier.
	"SYSLOG_IDENTIFIER": "syslog.identifier",
	// SYSLOG_PID maps to syslog.pid rather than the RFC5424 syslog.procid because
	// journald defines it as a numeric client PID, while RFC5424 PROCID is an
	// implementation-defined string that is not necessarily a process ID.
	"SYSLOG_PID":       "syslog.pid",
	"SYSLOG_TIMESTAMP": "syslog.timestamp",
}

// resourceMapping maps journald field names to OTel semantic convention resource attribute names.
// See: https://opentelemetry.io/docs/specs/semconv/registry/attributes/process/ and
// https://opentelemetry.io/docs/specs/semconv/registry/attributes/host/
var resourceMapping = map[string]string{
	"_HOSTNAME": "host.name",
	"_PID":      "process.pid",
	// _COMM is the base name of the executable as reported by /proc/[pid]/comm,
	// which systemd truncates to 15 characters.
	"_COMM":    "process.executable.name",
	"_EXE":     "process.executable.path",
	"_CMDLINE": "process.command_line",
}

// numericFields are OTel attribute/resource keys whose journald string values should be converted to int64.
var numericFields = map[string]bool{
	"code.line.number":     true,
	"syslog.facility.code": true,
	"syslog.pid":           true,
	"process.pid":          true,
}

// convertFieldValue converts a journald field value to the appropriate OTel type.
// For known numeric fields, it attempts to parse the string as int64.
func convertFieldValue(otelKey string, v any) any {
	if !numericFields[otelKey] {
		return v
	}
	s, ok := v.(string)
	if !ok {
		return v
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return v
	}
	return n
}

// mapJournalEntryAttributes processes the parsed journald entry body and applies
// OTel semantic convention attribute mappings. It:
//   - Sets entry.Body to the MESSAGE field value
//   - Sets entry.Severity and entry.SeverityText from the PRIORITY field
//   - Maps well-known fields to their OTel semantic convention attribute/resource names
//   - Puts remaining fields in entry.Attributes with "journald." prefix on their field names
func mapJournalEntryAttributes(e *entry.Entry, body map[string]any) {
	for k, v := range body {
		switch k {
		case "MESSAGE":
			e.Body = v
		case "PRIORITY":
			p, ok := v.(string)
			if !ok {
				break
			}
			if sev, ok := priorityToSeverity[p]; ok {
				e.Severity = sev
				e.SeverityText = priorityToSeverityText[p]
			}
		default:
			if attrKey, ok := attributeMapping[k]; ok {
				if e.Attributes == nil {
					e.Attributes = make(map[string]any)
				}
				e.Attributes[attrKey] = convertFieldValue(attrKey, v)
			} else if resKey, ok := resourceMapping[k]; ok {
				if e.Resource == nil {
					e.Resource = make(map[string]any)
				}
				e.Resource[resKey] = convertFieldValue(resKey, v)
			} else {
				if e.Attributes == nil {
					e.Attributes = make(map[string]any)
				}
				e.Attributes["journald."+k] = v
			}
		}
	}
}
