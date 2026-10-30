// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/transformconfig"
)

func TestToContextStatements(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		in := transformconfig.ContextStatements{Context: transformconfig.Span, Statements: []string{"a"}}
		got, err := toContextStatements(in)
		require.NoError(t, err)
		assert.Equal(t, &in, got)
	})

	t.Run("wrong type", func(t *testing.T) {
		_, err := toContextStatements("not a ContextStatements")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid context statements type")
	})
}
