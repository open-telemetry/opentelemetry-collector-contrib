// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicsyslogprocessor

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"go.opentelemetry.io/collector/model/pdata"
)

// tailSamplingSpanProcessor handles the incoming trace data and uses the given sampling
// policy to sample traces.
type sumologicSyslogProcessor struct {
	syslogFacilityAttrName string
	syslogFacilityRegex    *regexp.Regexp
}

const (
	syslogSource   = "syslog"
	facilityRegexp = `^<(?P<number>\d+)>`
)

var (
	facilities = map[int]string{
		0:  "kernel messages",
		1:  "user-level messages",
		2:  "mail system",
		3:  "system daemons",
		4:  "security/authorization messages",
		5:  "messages generated internally by syslogd",
		6:  "line printer subsystem",
		7:  "network news subsystem",
		8:  "UUCP subsystem",
		9:  "clock daemon",
		10: "security/authorization messages",
		11: "FTP daemon",
		12: "NTP subsystem",
		13: "log audit",
		14: "log alert",
		15: "clock daemon",
		16: "local use 0  (local0)",
		17: "local use 1  (local1)",
		18: "local use 2  (local2)",
		19: "local use 3  (local3)",
		20: "local use 4  (local4)",
		21: "local use 5  (local5)",
		22: "local use 6  (local6)",
		23: "local use 7  (local7)",
	}
)

func newSumologicSyslogProcessor(cfg *Config) (*sumologicSyslogProcessor, error) {
	r, err := regexp.Compile(facilityRegexp)
	if err != nil {
		return nil, err
	}

	return &sumologicSyslogProcessor{
		syslogFacilityAttrName: cfg.FacilityAttr,
		syslogFacilityRegex:    r,
	}, nil
}

// ProcessLogs tries to extract facility number from log syslog line and maps it to facility name.
// Facility is taken as $number/8 rounded down, where log looks like `^<$number> .*`
func (ssp *sumologicSyslogProcessor) ProcessLogs(ctx context.Context, ld pdata.Logs) (pdata.Logs, error) {
	// Iterate over ResourceLogs
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)

		ills := rl.InstrumentationLibraryLogs()
		// iterate over InstrumentationLibraryLogs
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)

			// iterate over Logs
			logs := ill.Logs()
			for k := 0; k < logs.Len(); k++ {
				var (
					value = syslogSource
					ok    bool
				)

				log := logs.At(k)
				match := ssp.syslogFacilityRegex.FindStringSubmatch(log.Body().StringVal())

				if match != nil {
					facility, err := strconv.Atoi(match[1])
					if err != nil {
						return ld, fmt.Errorf("failed to parse: %s, err: %w", match[1], err)
					}
					facility = facility / 8

					value, ok = facilities[facility]
					if !ok {
						value = syslogSource
					}
				}
				log.Attributes().UpsertString(ssp.syslogFacilityAttrName, value)
			}
		}
	}

	return ld, nil
}
