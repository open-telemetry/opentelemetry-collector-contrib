// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	assert.Equal(t, &Config{MaxConcurrentPartitions: defaultMaxConcurrentPartitions()}, cfg)
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

	pp, err := f.CreateProfiles(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, pp)
}
