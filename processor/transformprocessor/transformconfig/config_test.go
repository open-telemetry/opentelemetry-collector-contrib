// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextID_UnmarshalText(t *testing.T) {
	valid := []ContextID{Resource, Scope, Span, SpanEvent, Metric, DataPoint, Exemplar, Log, Profile}
	for _, want := range valid {
		t.Run(string(want), func(t *testing.T) {
			var got ContextID
			require.NoError(t, got.UnmarshalText([]byte(want)))
			assert.Equal(t, want, got)
		})
	}

	t.Run("normalizes case", func(t *testing.T) {
		var got ContextID
		require.NoError(t, got.UnmarshalText([]byte("SPAN")))
		assert.Equal(t, Span, got)
	})

	t.Run("rejects unknown context", func(t *testing.T) {
		var got ContextID
		err := got.UnmarshalText([]byte("nonsense"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown context")
	})
}

func TestContextStatements_GetStatements(t *testing.T) {
	cs := ContextStatements{Statements: []string{"a", "b"}}
	assert.Equal(t, []string{"a", "b"}, cs.GetStatements())
}
