// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"
)

// TestReadmeConditionExamples parses every OTTL condition used in the README so
// documentation examples cannot silently drift from what the processor actually
// accepts.
func TestReadmeConditionExamples(t *testing.T) {
	examples := []string{
		`span.status.code == STATUS_CODE_ERROR`,
		`span.kind == SPAN_KIND_SERVER and span.attributes["http.response.status_code"] >= 500`,
		`IsRootSpan() and resource.attributes["service.name"] == "checkout"`,
		`resource.attributes["service.name"] == "checkout" or resource.attributes["service.name"] == "billing"`,
		`IsMatch(span.name, "^GET /api/")`,
	}
	for _, expr := range examples {
		t.Run(expr, func(t *testing.T) {
			_, err := compileRule(&RuleConfig{
				Name:       "test",
				Conditions: []string{expr},
			}, sampler.NewAlwaysSample(), nil, testSettings(), nil)
			assert.NoError(t, err, "example failed to compile: %s", expr)
		})
	}
}

// TestReadmeRootSpanConditionExamples parses every OTTL expression shown in the
// README's Root-span detection section, so those recipes stay compilable.
func TestReadmeRootSpanConditionExamples(t *testing.T) {
	examples := []string{
		`IsRootSpan()`,
		`IsRootSpan() or span.attributes["otelcol.adaptive_tail_sampling.root_span"] == true`,
		`IsRootSpan() or (span.kind == SPAN_KIND_SERVER and resource.attributes["service.name"] == "gateway")`,
		`IsRootSpan() or (span.kind == SPAN_KIND_CONSUMER and IsMatch(span.name, "^receive "))`,
	}
	for _, expr := range examples {
		t.Run(expr, func(t *testing.T) {
			_, err := compileRootSpanCondition(expr, testSettings())
			assert.NoError(t, err, "example failed to compile: %s", expr)
		})
	}
}
