// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package journald // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"

import (
	"path"
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
// _COMM is deliberately not mapped here: it is the value of /proc/[pid]/comm, which does
// not reliably match process.executable.name. It is kept as journald._COMM instead
var resourceMapping = map[string]string{
	"_HOSTNAME": "host.name",
	"_PID":      "process.pid",
	"_EXE":      "process.executable.path",
	"_CMDLINE":  "process.command_line",
}

// numericFields are OTel attribute/resource keys whose journald string values should be converted to int64.
var numericFields = map[string]bool{
	"code.line.number":     true,
	"syslog.facility.code": true,
	"syslog.pid":           true,
	"process.pid":          true,
}

// convertFieldValue converts a journald field value to the type required by the OTel
// attribute it is mapped to. For known numeric fields the string value is parsed as int64.
// It reports false when the value cannot be converted, in which case the caller must not
// use the semantic convention key, so that a typed attribute never holds a wrong type.
func convertFieldValue(otelKey string, v any) (any, bool) {
	if !numericFields[otelKey] {
		return v, true
	}
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, false
	}
	return n, true
}

// mapJournalEntryAttributes processes the parsed journald entry body and applies
// OTel semantic convention attribute mappings. It:
//   - Sets entry.Body to the MESSAGE field value
//   - Sets entry.Severity and entry.SeverityText from the PRIORITY field
//   - Maps well-known fields to their OTel semantic convention attribute/resource names
//   - Derives process.executable.name from the base name of _EXE
//   - Puts remaining fields in entry.Attributes with "journald." prefix on their field names
//
// Fields whose value cannot be converted to the type required by the semantic convention
// are kept unconverted under their "journald." prefixed field name instead.
func mapJournalEntryAttributes(e *entry.Entry, body map[string]any) {
	// Clear the raw journal record set by NewEntry; it is only used there so that
	// EXPR-based attributes/resource config can reference journal fields via `body`.
	e.Body = nil

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
				if cv, ok := convertFieldValue(attrKey, v); ok {
					if e.Attributes == nil {
						e.Attributes = make(map[string]any)
					}
					e.Attributes[attrKey] = cv
					continue
				}
			} else if resKey, ok := resourceMapping[k]; ok {
				if cv, ok := convertFieldValue(resKey, v); ok {
					if e.Resource == nil {
						e.Resource = make(map[string]any)
					}
					e.Resource[resKey] = cv
					continue
				}
			}
			// Unmapped field, or a value that does not fit the type required by the
			// semantic convention: keep the original field name and value.
			if e.Attributes == nil {
				e.Attributes = make(map[string]any)
			}
			e.Attributes["journald."+k] = v
		}
	}

	// process.executable.name is the base name of the target of /proc/[pid]/exe, which
	// journald reports as _EXE.
	if exe, ok := e.Resource["process.executable.path"].(string); ok && exe != "" {
		e.Resource["process.executable.name"] = path.Base(exe)
	}
}
