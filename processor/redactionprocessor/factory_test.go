// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/metadata"
)

func TestDefaultConfiguration(t *testing.T) {
	c := createDefaultConfig().(*Config)
	assert.Empty(t, c.AllowedKeys)
	assert.Empty(t, c.BlockedValues)
	assert.Empty(t, c.AllowedValues)
	assert.Empty(t, c.BlockedKeyPatterns)
	assert.Empty(t, c.HashFunction)
}

func TestCreateTestProcessor(t *testing.T) {
	cfg := &Config{}

	tp, err := createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.True(t, tp.Capabilities().MutatesData)
}

func TestCreateTestLogsProcessor(t *testing.T) {
	cfg := &Config{}

	tp, err := createLogsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.True(t, tp.Capabilities().MutatesData)
}

func TestCreateTestMetricsProcessor(t *testing.T) {
	cfg := &Config{}

	tp, err := createMetricsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.True(t, tp.Capabilities().MutatesData)
}
