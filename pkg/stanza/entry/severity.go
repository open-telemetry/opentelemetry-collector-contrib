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
	Default: "default",
	Trace:   "trace",
	Trace2:  "trace2",
	Trace3:  "trace3",
	Trace4:  "trace4",
	Debug:   "debug",
	Debug2:  "debug2",
	Debug3:  "debug3",
	Debug4:  "debug4",
	Info:    "info",
	Info2:   "info2",
	Info3:   "info3",
	Info4:   "info4",
	Warn:    "warn",
	Warn2:   "warn2",
	Warn3:   "warn3",
	Warn4:   "warn4",
	Error:   "error",
	Error2:  "error2",
	Error3:  "error3",
	Error4:  "error4",
	Fatal:   "fatal",
	Fatal2:  "fatal2",
	Fatal3:  "fatal3",
	Fatal4:  "fatal4",
}

// ToString converts a severity to a string
func (s Severity) String() string {
	if str, ok := sevText[s]; ok {
		return str
	}
	return strconv.Itoa(int(s))
}
