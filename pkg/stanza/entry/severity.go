// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

import (
	"strconv"
)

// Severity indicates the seriousness of a log entry
type Severity int

const (
	// Default indicates an unknown severity
	Default Severity = iota

	// A fine-grained debugging event. Typically disabled in default configurations.
	Trace
	Trace2
	Trace3
	Trace4

	// A debugging event.
	Debug
	Debug2
	Debug3
	Debug4

	// An informational event. Indicates that an event happened.
	Info
	Info2
	Info3
	Info4

	// A warning event. Not an error but is likely more important than an informational event.
	Warn
	Warn2
	Warn3
	Warn4

	// An error event. Something went wrong.
	Error
	Error2
	Error3
	Error4

	// An error event. Something went wrong.
	Fatal
	Fatal2
	Fatal3
	Fatal4
)

var sevText = map[Severity]string{
	Default: "DEFAULT",
	Trace:   "TRACE",
	Trace2:  "TRACE2",
	Trace3:  "TRACE3",
	Trace4:  "TRACE4",
	Debug:   "DEBUG",
	Debug2:  "DEBUG2",
	Debug3:  "DEBUG3",
	Debug4:  "DEBUG4",
	Info:    "INFO",
	Info2:   "INFO2",
	Info3:   "INFO3",
	Info4:   "INFO4",
	Warn:    "WARN",
	Warn2:   "WARN2",
	Warn3:   "WARN3",
	Warn4:   "WARN4",
	Error:   "ERROR",
	Error2:  "ERROR2",
	Error3:  "ERROR3",
	Error4:  "ERROR4",
	Fatal:   "FATAL",
	Fatal2:  "FATAL2",
	Fatal3:  "FATAL3",
	Fatal4:  "FATAL4",
}

// ToString converts a severity to a string
func (s Severity) String() string {
	if str, ok := sevText[s]; ok {
		return str
	}
	return strconv.Itoa(int(s))
}
