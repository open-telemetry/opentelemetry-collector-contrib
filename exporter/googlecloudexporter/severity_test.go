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
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	sev "google.golang.org/genproto/googleapis/logging/type"
)

func TestConvertSeverity(t *testing.T) {
	testCases := []struct {
		name         string
		origSeverity pdata.SeverityNumber
		newSeverity  sev.LogSeverity
	}{
		{
			name:         "original severity above FATAL",
			origSeverity: pdata.SeverityNumberFATAL4,
			newSeverity:  sev.LogSeverity_CRITICAL,
		},
		{
			name:         "original severity equals FATAL",
			origSeverity: pdata.SeverityNumberFATAL,
			newSeverity:  sev.LogSeverity_CRITICAL,
		},
		{
			name:         "original severity above ERROR",
			origSeverity: pdata.SeverityNumberERROR4,
			newSeverity:  sev.LogSeverity_ERROR,
		},
		{
			name:         "original severity equals ERROR",
			origSeverity: pdata.SeverityNumberERROR,
			newSeverity:  sev.LogSeverity_ERROR,
		},
		{
			name:         "original severity above WARN",
			origSeverity: pdata.SeverityNumberWARN4,
			newSeverity:  sev.LogSeverity_WARNING,
		},
		{
			name:         "original severity equals WARN",
			origSeverity: pdata.SeverityNumberWARN4,
			newSeverity:  sev.LogSeverity_WARNING,
		},
		{
			name:         "original severity above INFO",
			origSeverity: pdata.SeverityNumberINFO4,
			newSeverity:  sev.LogSeverity_INFO,
		},
		{
			name:         "original severity equals INFO",
			origSeverity: pdata.SeverityNumberINFO,
			newSeverity:  sev.LogSeverity_INFO,
		},
		{
			name:         "original severity above DEBUG",
			origSeverity: pdata.SeverityNumberDEBUG4,
			newSeverity:  sev.LogSeverity_DEBUG,
		},
		{
			name:         "original severity equals DEBUG",
			origSeverity: pdata.SeverityNumberDEBUG,
			newSeverity:  sev.LogSeverity_DEBUG,
		},
		{
			name:         "original severity above TRACE",
			origSeverity: pdata.SeverityNumberTRACE4,
			newSeverity:  sev.LogSeverity_DEBUG,
		},
		{
			name:         "original severity equals TRACE",
			origSeverity: pdata.SeverityNumberTRACE,
			newSeverity:  sev.LogSeverity_DEBUG,
		},
		{
			name:         "original severity equals UNDEFINED",
			origSeverity: pdata.SeverityNumberUNDEFINED,
			newSeverity:  sev.LogSeverity_DEFAULT,
		},
		{
			name:         "original severity not recognized",
			origSeverity: -10,
			newSeverity:  sev.LogSeverity_DEFAULT,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := convertSeverity(tc.origSeverity)
			require.Equal(t, tc.newSeverity, result)
		})
	}
}
