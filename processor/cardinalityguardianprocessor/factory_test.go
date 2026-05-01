// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, component.MustNewType("cardinality_guardian"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)

	// Ensure cast to Config works and validates cleanly
	oCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.NoError(t, oCfg.Validate())
}

func TestCreateMetricsProcessor(t *testing.T) {
	cfg := createDefaultConfig()
	set := processortest.NewNopSettings(component.MustNewType("cardinality_guardian"))

	// Test successful creation
	tp, err := createMetricsProcessor(t.Context(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	// Test invalid config type
	_, err = createMetricsProcessor(t.Context(), set, nil, consumertest.NewNop())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config type")
}
