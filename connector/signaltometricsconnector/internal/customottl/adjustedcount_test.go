package customottl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

func Test_AdjustedCount(t *testing.T) {
	for _, tc := range []struct {
		tracestate string
		want       float64
		errMsg     string
	}{
		{tracestate: "", want: 1},
		{tracestate: "invalid=p:8;th:8", want: 1},           // otel trace state nil, default to 1
		{tracestate: "ot=notfound:8", want: 1},              // otel tvalue 0, default to 1
		{tracestate: "ot=404:0", errMsg: "failed to parse"}, // invalid syntax
		{tracestate: "ot=th:0", want: 1},                    // 100% sampling
		{tracestate: "ot=th:8", want: 2},                    // 50% sampling
		{tracestate: "ot=th:c", want: 4},                    // 25% sampling
	} {
		t.Run("tracestate/"+tc.tracestate, func(t *testing.T) {
			exprFunc, err := adjustedCount()
			require.NoError(t, err)
			span := ptrace.NewSpan()
			span.TraceState().FromRaw(tc.tracestate)
			result, err := exprFunc(nil, ottlspan.NewTransformContext(
				span,
				pcommon.NewInstrumentationScope(),
				pcommon.NewResource(),
				ptrace.NewScopeSpans(),
				ptrace.NewResourceSpans(),
			))
			if tc.errMsg != "" {
				require.ErrorContains(t, err, tc.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}
}
