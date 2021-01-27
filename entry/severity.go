// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entry

import (
	"strconv"
)

// Severity indicates the seriousness of a log entry
type Severity int

var namedLevels = map[Severity]string{
	Default:     "default",
	Trace:       "trace",
	Trace2:      "trace2",
	Trace3:      "trace3",
	Trace4:      "trace4",
	Debug:       "debug",
	Debug2:      "debug2",
	Debug3:      "debug3",
	Debug4:      "debug4",
	Info:        "info",
	Info2:       "info2",
	Info3:       "info3",
	Info4:       "info4",
	Notice:      "notice",
	Warning:     "warning",
	Warning2:    "warning2",
	Warning3:    "warning3",
	Warning4:    "warning4",
	Error:       "error",
	Error2:      "error2",
	Error3:      "error3",
	Error4:      "error4",
	Critical:    "critical",
	Alert:       "alert",
	Emergency:   "emergency",
	Emergency2:  "emergency2",
	Emergency3:  "emergency3",
	Emergency4:  "emergency4",
	Catastrophe: "catastrophe",
}

// ToString converts a severity to a string
func (s Severity) String() string {
	if str, ok := namedLevels[s]; ok {
		return str
	}
	return strconv.Itoa(int(s))
}

const (
	// Default indicates an unknown severity
	Default Severity = 0

	// Trace indicates that the log may be useful for detailed debugging
	Trace Severity = 10

	// Trace2 indicates that the log may be useful for detailed debugging
	Trace2 Severity = 12

	// Trace3 indicates that the log may be useful for detailed debugging
	Trace3 Severity = 13

	// Trace4 indicates that the log may be useful for detailed debugging
	Trace4 Severity = 14

	// Debug indicates that the log may be useful for debugging purposes
	Debug Severity = 20

	// Debug2 indicates that the log may be useful for debugging purposes
	Debug2 Severity = 22

	// Debug3 indicates that the log may be useful for debugging purposes
	Debug3 Severity = 23

	// Debug4 indicates that the log may be useful for debugging purposes
	Debug4 Severity = 24

	// Info indicates that the log may be useful for understanding high level details about an application
	Info Severity = 30

	// Info2 indicates that the log may be useful for understanding high level details about an application
	Info2 Severity = 32

	// Info3 indicates that the log may be useful for understanding high level details about an application
	Info3 Severity = 33

	// Info4 indicates that the log may be useful for understanding high level details about an application
	Info4 Severity = 34

	// Notice indicates that the log should be noticed
	Notice Severity = 40

	// Warning indicates that someone should look into an issue
	Warning Severity = 50

	// Warning2 indicates that someone should look into an issue
	Warning2 Severity = 52

	// Warning3 indicates that someone should look into an issue
	Warning3 Severity = 53

	// Warning4 indicates that someone should look into an issue
	Warning4 Severity = 54

	// Error indicates that something undesirable has actually happened
	Error Severity = 60

	// Error2 indicates that something undesirable has actually happened
	Error2 Severity = 62

	// Error3 indicates that something undesirable has actually happened
	Error3 Severity = 63

	// Error4 indicates that something undesirable has actually happened
	Error4 Severity = 64

	// Critical indicates that a problem requires attention immediately
	Critical Severity = 70

	// Alert indicates that action must be taken immediately
	Alert Severity = 80

	// Emergency indicates that the application is unusable
	Emergency Severity = 90

	// Emergency2 indicates that the application is unusable
	Emergency2 Severity = 92

	// Emergency3 indicates that the application is unusable
	Emergency3 Severity = 93

	// Emergency4 indicates that the application is unusable
	Emergency4 Severity = 94

	// Catastrophe indicates that it is already too late
	Catastrophe Severity = 100
)
