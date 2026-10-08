// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
)

func validConfig() *Config {
	return &Config{Keys: map[string]string{
		"tenant_id": `resource.attributes["tenant.id"]`,
	}}
}

func nopSettings() processor.Settings {
	return processortest.NewNopSettings(NewFactory().Type())
}

func TestCreateLogsProcessor_ValidConfig(t *testing.T) {
	p, err := NewFactory().CreateLogs(t.Context(), nopSettings(), validConfig(), consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestCreateLogsProcessor_InvalidOTTL(t *testing.T) {
	cfg := &Config{Keys: map[string]string{"bad": "not_a_valid_expression("}}
	_, err := NewFactory().CreateLogs(t.Context(), nopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
}

func TestCreateTracesProcessor_ValidConfig(t *testing.T) {
	f := NewFactory()
	p, err := f.CreateTraces(t.Context(), nopSettings(), validConfig(), consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestCreateTracesProcessor_InvalidOTTL(t *testing.T) {
	f := NewFactory()
	cfg := &Config{Keys: map[string]string{"bad": "not_a_valid_expression("}}
	_, err := f.CreateTraces(t.Context(), nopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessors(t *testing.T) {
	f := NewFactory()
	set := processortest.NewNopSettings(f.Type())
	cfg := &Config{Keys: map[string]string{"tenant_id": `resource.attributes["tenant.id"]`}}

	lp, err := f.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, lp)

	mp, err := f.CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, mp)

	tp, err := f.CreateTraces(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, tp)

	pp, err := f.(xprocessor.Factory).CreateProfiles(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, pp)
}
