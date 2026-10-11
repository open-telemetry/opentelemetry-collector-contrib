// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"
)

func testSettings() component.TelemetrySettings {
	return component.TelemetrySettings{Logger: zap.NewNop()}
}

func mustRule(t *testing.T, cfg RuleConfig) *rule {
	t.Helper()
	r, err := compileRule(&cfg, sampler.NewAlwaysSample(), nil, testSettings(), nil)
	require.NoError(t, err)
	return r
}

// twoSpanTrace builds a two-span trace: the first span is on service "api"
// with status ERROR, the second span is on service "payment" with status OK.
// Resource attributes are set on the first ResourceSpans only.
func twoSpanTrace() []ptrace.ResourceSpans {
	td := ptrace.NewTraces()
	rs1 := td.ResourceSpans().AppendEmpty()
	rs1.Resource().Attributes().PutStr("service.name", "api")
	span1 := rs1.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetName("GET /users")
	span1.Attributes().PutStr("http.method", "GET")
	span1.Status().SetCode(ptrace.StatusCodeError)

	rs2 := td.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutStr("service.name", "payment")
	span2 := rs2.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetName("POST /charge")
	span2.Attributes().PutStr("http.method", "POST")
	span2.Status().SetCode(ptrace.StatusCodeOk)

	return []ptrace.ResourceSpans{rs1, rs2}
}

func TestRule_EmptyConditionsAlwaysMatches(t *testing.T) {
	r := mustRule(t, RuleConfig{Name: "catchall"})
	assert.True(t, r.matches(t.Context(), twoSpanTrace()))
}

func TestRule_AnySpan_EachConditionIndependent(t *testing.T) {
	// Under any_span, condition A (error status) is satisfied by span 1;
	// condition B (payment service) is satisfied by span 2. Different spans;
	// rule should still match.
	r := mustRule(t, RuleConfig{
		Name: "cross-span",
		Conditions: []string{
			`span.status.code == STATUS_CODE_ERROR`,
			`resource.attributes["service.name"] == "payment"`,
		},
		Match: MatchAnySpan,
	})
	assert.True(t, r.matches(t.Context(), twoSpanTrace()))
}

func TestRule_SameSpan_RejectsCrossSpanMatches(t *testing.T) {
	// Same conditions as above but same_span requires one span to satisfy both.
	// No single span in twoSpanTrace is both an error AND on payment service,
	// so this must NOT match.
	r := mustRule(t, RuleConfig{
		Name: "co-located",
		Conditions: []string{
			`span.status.code == STATUS_CODE_ERROR`,
			`resource.attributes["service.name"] == "payment"`,
		},
		Match: MatchSameSpan,
	})
	assert.False(t, r.matches(t.Context(), twoSpanTrace()))
}

func TestRule_SameSpan_AcceptsCoLocatedMatch(t *testing.T) {
	// The api-service error span satisfies both conditions.
	r := mustRule(t, RuleConfig{
		Name: "api-errors",
		Conditions: []string{
			`span.status.code == STATUS_CODE_ERROR`,
			`resource.attributes["service.name"] == "api"`,
		},
		Match: MatchSameSpan,
	})
	assert.True(t, r.matches(t.Context(), twoSpanTrace()))
}

func TestRule_AnySpan_IsDefault(t *testing.T) {
	// Match unset → any_span default. Same behavior as the explicit any_span
	// case above.
	r := mustRule(t, RuleConfig{
		Name: "default-match",
		Conditions: []string{
			`span.status.code == STATUS_CODE_ERROR`,
			`resource.attributes["service.name"] == "payment"`,
		},
	})
	assert.Equal(t, MatchAnySpan, r.matchMode)
	assert.True(t, r.matches(t.Context(), twoSpanTrace()))
}

func TestCompileRule_RejectsInvalidOTTL(t *testing.T) {
	_, err := compileRule(&RuleConfig{
		Name:       "bad",
		Conditions: []string{`this is not ottl syntax`},
	}, sampler.NewAlwaysSample(), nil, testSettings(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
}
