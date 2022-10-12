// Copyright The OpenTelemetry Authors
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

package filterlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestSeverityMatcher_MatchLogRecord(t *testing.T) {
	testCases := []struct {
		name           string
		minSeverity    plog.SeverityNumber
		matchUndefined bool
		inputSeverity  plog.SeverityNumber
		matches        bool
	}{
		{
			name:          "INFO matches if TRACE is min",
			minSeverity:   plog.SeverityNumberTrace,
			inputSeverity: plog.SeverityNumberInfo,
			matches:       true,
		},
		{
			name:          "INFO matches if INFO is min",
			minSeverity:   plog.SeverityNumberInfo,
			inputSeverity: plog.SeverityNumberInfo,
			matches:       true,
		},
		{
			name:          "INFO does not match if WARN is min",
			minSeverity:   plog.SeverityNumberWarn,
			inputSeverity: plog.SeverityNumberInfo,
			matches:       false,
		},
		{
			name:          "INFO does not match if INFO2 is min",
			minSeverity:   plog.SeverityNumberInfo2,
			inputSeverity: plog.SeverityNumberInfo,
			matches:       false,
		},
		{
			name:          "INFO2 matches if INFO is min",
			minSeverity:   plog.SeverityNumberInfo,
			inputSeverity: plog.SeverityNumberInfo2,
			matches:       true,
		},
		{
			name:          "UNDEFINED does not match if TRACE is min",
			minSeverity:   plog.SeverityNumberTrace,
			inputSeverity: plog.SeverityNumberUnspecified,
			matches:       false,
		},
		{
			name:          "UNDEFINED does not match if UNDEFINED is min",
			minSeverity:   plog.SeverityNumberUnspecified,
			inputSeverity: plog.SeverityNumberUnspecified,
			matches:       false,
		},
		{
			name:           "UNDEFINED matches if matchUndefined is true",
			minSeverity:    plog.SeverityNumberUnspecified,
			matchUndefined: true,
			inputSeverity:  plog.SeverityNumberUnspecified,
			matches:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := newSeverityNumberMatcher(tc.minSeverity, tc.matchUndefined)

			r := pcommon.NewResource()
			i := pcommon.NewInstrumentationScope()
			lr := plog.NewLogRecord()
			lr.SetSeverityNumber(tc.inputSeverity)

			require.Equal(t, tc.matches, matcher.MatchLogRecord(lr, r, i))
		})
	}
}
