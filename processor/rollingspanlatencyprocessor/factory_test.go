// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rollingspanlatencyprocessor

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
	assert.Equal(t, component.MustNewType("rolling_span_latency"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)

	oCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.NoError(t, oCfg.Validate())
}

func TestCreateTracesProcessor(t *testing.T) {
	cfg := createDefaultConfig()
	set := processortest.NewNopSettings(component.MustNewType("rolling_span_latency"))

	tp, err := createTracesProcessor(t.Context(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	_, err = createTracesProcessor(t.Context(), set, nil, consumertest.NewNop())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config type")
}
