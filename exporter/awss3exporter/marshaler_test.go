// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMarshaler(t *testing.T) {
	{
		m, err := newMarshaler("otlp_json", zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, m.format(), "json")
	}
	{
		m, err := newMarshaler("sumo_ic", zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, m.format(), "json.gz")
	}
	{
		m, err := newMarshaler("unknown", zap.NewNop())
		assert.Error(t, err)
		require.Nil(t, m)
	}
}
