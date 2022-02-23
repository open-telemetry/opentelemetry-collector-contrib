// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudexporter

import (
	"go.opentelemetry.io/collector/model/pdata"
	sev "google.golang.org/genproto/googleapis/logging/type"
)

var fastSev = map[pdata.SeverityNumber]sev.LogSeverity{
	pdata.SeverityNumberFATAL:     sev.LogSeverity_CRITICAL,
	pdata.SeverityNumberERROR:     sev.LogSeverity_ERROR,
	pdata.SeverityNumberWARN:      sev.LogSeverity_WARNING,
	pdata.SeverityNumberINFO:      sev.LogSeverity_INFO,
	pdata.SeverityNumberDEBUG:     sev.LogSeverity_DEBUG,
	pdata.SeverityNumberTRACE:     sev.LogSeverity_DEBUG,
	pdata.SeverityNumberUNDEFINED: sev.LogSeverity_DEFAULT,
}

// convertSeverity converts from otel's severity to google's severity levels
func convertSeverity(s pdata.SeverityNumber) sev.LogSeverity {
	if logSev, ok := fastSev[s]; ok {
		return logSev
	}

	switch {
	case s >= pdata.SeverityNumberFATAL:
		return sev.LogSeverity_CRITICAL
	case s >= pdata.SeverityNumberERROR:
		return sev.LogSeverity_ERROR
	case s >= pdata.SeverityNumberWARN:
		return sev.LogSeverity_WARNING
	case s >= pdata.SeverityNumberINFO:
		return sev.LogSeverity_INFO
	case s > pdata.SeverityNumberTRACE:
		return sev.LogSeverity_DEBUG
	default:
		return sev.LogSeverity_DEFAULT
	}
}
