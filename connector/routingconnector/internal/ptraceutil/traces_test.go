// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptraceutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestMoveResourcesIf(t *testing.T) {
	testCases := []struct {
		name       string
		moveIf     func(ptrace.ResourceSpans) bool
		from       ptrace.Traces
		to         ptrace.Traces
		expectFrom ptrace.Traces
		expectTo   ptrace.Traces
	}{
		{
			name: "move_none",
			moveIf: func(ptrace.ResourceSpans) bool {
				return false
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			to:         ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			expectTo:   ptrace.NewTraces(),
		},
		{
			name: "move_all",
			moveIf: func(ptrace.ResourceSpans) bool {
				return true
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			to:         ptrace.NewTraces(),
			expectFrom: ptrace.NewTraces(),
			expectTo:   ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
		},
		{
			name: "move_one",
			moveIf: func(rl ptrace.ResourceSpans) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceA"
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			to:         ptrace.NewTraces(),
			expectFrom: ptraceutiltest.NewTraces("B", "CD", "EF", "FG"),
			expectTo:   ptraceutiltest.NewTraces("A", "CD", "EF", "FG"),
		},
		{
			name: "move_to_preexisting",
			moveIf: func(rl ptrace.ResourceSpans) bool {
				rname, ok := rl.Resource().Attributes().Get("resourceName")
				return ok && rname.AsString() == "resourceB"
			},
			from:       ptraceutiltest.NewTraces("AB", "CD", "EF", "FG"),
			to:         ptraceutiltest.NewTraces("1", "2", "3", "4"),
			expectFrom: ptraceutiltest.NewTraces("A", "CD", "EF", "FG"),
			expectTo: func() ptrace.Traces {
				move := ptraceutiltest.NewTraces("B", "CD", "EF", "FG")
				moveTo := ptraceutiltest.NewTraces("1", "2", "3", "4")
				move.ResourceSpans().MoveAndAppendTo(moveTo.ResourceSpans())
				return moveTo
			}(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ptraceutil.MoveResourcesIf(tt.from, tt.to, tt.moveIf)
			assert.NoError(t, ptracetest.CompareTraces(tt.expectFrom, tt.from), "from not modified as expected")
			assert.NoError(t, ptracetest.CompareTraces(tt.expectTo, tt.to), "to not as expected")
		})
	}
}
