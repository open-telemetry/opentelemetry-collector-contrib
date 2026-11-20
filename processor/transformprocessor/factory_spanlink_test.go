// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func TestFactoryCreateSpanLinkProcessor(t *testing.T) {
	gate := metadata.ProcessorTransformEnableExperimentalFunctionsFeatureGate
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	t.Cleanup(func() { require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), false)) })

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.TraceStatements = []common.ContextStatements{{
		Context:    "spanlink",
		Conditions: []string{`span.name == "operation"`},
		Statements: []string{`set(attributes["checked"], true)`},
	}}
	require.NoError(t, cfg.Validate())

	mp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("operation")
	link := span.Links().AppendEmpty()
	require.NoError(t, mp.ConsumeTraces(t.Context(), td))

	checked, ok := link.Attributes().Get("checked")
	require.True(t, ok)
	assert.True(t, checked.Bool())
}
