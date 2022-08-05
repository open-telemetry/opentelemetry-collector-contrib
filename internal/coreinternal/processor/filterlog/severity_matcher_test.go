package filterlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestSeverityMatcher_MatchLogRecord(t *testing.T) {
	testCases := []struct {
		name          string
		minSeverity   plog.SeverityNumber
		inputSeverity plog.SeverityNumber
		matches       bool
	}{
		{
			name:          "INFO matches if TRACE is min",
			minSeverity:   plog.SeverityNumberTRACE,
			inputSeverity: plog.SeverityNumberINFO,
			matches:       true,
		},
		{
			name:          "INFO matches if INFO is min",
			minSeverity:   plog.SeverityNumberINFO,
			inputSeverity: plog.SeverityNumberINFO,
			matches:       true,
		},
		{
			name:          "INFO does not match if WARN is min",
			minSeverity:   plog.SeverityNumberWARN,
			inputSeverity: plog.SeverityNumberINFO,
			matches:       false,
		},
		{
			name:          "INFO does not match if INFO2 is min",
			minSeverity:   plog.SeverityNumberINFO2,
			inputSeverity: plog.SeverityNumberINFO,
			matches:       false,
		},
		{
			name:          "INFO2 matches if INFO is min",
			minSeverity:   plog.SeverityNumberINFO,
			inputSeverity: plog.SeverityNumberINFO2,
			matches:       true,
		},
		{
			name:          "UNDEFINED does not match if TRACE is min",
			minSeverity:   plog.SeverityNumberTRACE,
			inputSeverity: plog.SeverityNumberUNDEFINED,
			matches:       false,
		},
		{
			name:          "UNDEFINED does not match if UNDEFINED is min",
			minSeverity:   plog.SeverityNumberUNDEFINED,
			inputSeverity: plog.SeverityNumberUNDEFINED,
			matches:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := newSeverityNumberMatcher(tc.minSeverity)

			r := pcommon.NewResource()
			i := pcommon.NewInstrumentationScope()
			lr := plog.NewLogRecord()
			lr.SetSeverityNumber(tc.inputSeverity)

			require.Equal(t, tc.matches, matcher.MatchLogRecord(lr, r, i))
		})
	}
}
