// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func newLogParserCollection(t *testing.T) *LogParserCollection {
	t.Helper()
	pc, err := NewLogParserCollection(
		componenttest.NewNopTelemetrySettings(),
		WithLogParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext]()),
		WithLogErrorMode(ottl.PropagateError),
	)
	require.NoError(t, err)
	return pc
}

func TestLogParserCollection_ConsumeLogs(t *testing.T) {
	tests := []struct {
		name string
		cs   ContextStatements
	}{
		{"log context", ContextStatements{Context: Log, Statements: []string{`set(attributes["test"], "pass")`}}},
		{"log context with condition", ContextStatements{Context: Log, Conditions: []string{`body == "body"`}, Statements: []string{`set(attributes["test"], "pass")`}}},
		{"inferred context", ContextStatements{Statements: []string{`set(log.attributes["test"], "pass")`}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := newLogParserCollection(t)
			consumer, err := pc.ParseContextStatements(tt.cs)
			require.NoError(t, err)
			assert.Equal(t, Log, consumer.Context())

			ld := newTestLogs()
			require.NoError(t, consumer.ConsumeLogs(t.Context(), ld, nil))

			v, ok := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("test")
			require.True(t, ok)
			assert.Equal(t, "pass", v.Str())
		})
	}
}

func TestLogParserCollection_ConsumeLogs_PropagatesError(t *testing.T) {
	pc := newLogParserCollection(t)
	consumer, err := pc.ParseContextStatements(ContextStatements{Context: Log, Statements: []string{`set(attributes["test"], ParseJSON("1"))`}})
	require.NoError(t, err)
	require.Error(t, consumer.ConsumeLogs(t.Context(), newTestLogs(), nil))
}

func TestLogParserCollection_ParseContextStatements_Error(t *testing.T) {
	pc := newLogParserCollection(t)
	_, err := pc.ParseContextStatements(ContextStatements{Context: Log, Statements: []string{`not a valid statement`}})
	require.Error(t, err)
}
