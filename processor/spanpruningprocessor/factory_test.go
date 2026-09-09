// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	oCfg := cfg.(*Config)
	assert.Equal(t, 5, oCfg.MinSpansToAggregate)
	assert.Equal(t, "aggregation.", oCfg.AggregationAttributePrefix)
	assert.False(t, oCfg.EnableOutlierAnalysis)
	assert.Equal(t, OutlierMethodIQR, oCfg.OutlierAnalysis.Method)
	assert.Equal(t, 1.5, oCfg.OutlierAnalysis.IQRMultiplier)
	assert.Equal(t, 3.0, oCfg.OutlierAnalysis.MADMultiplier)
	assert.Equal(t, 7, oCfg.OutlierAnalysis.MinGroupSize)
	assert.Equal(t, 0.75, oCfg.OutlierAnalysis.CorrelationMinOccurrence)
	assert.Equal(t, 0.25, oCfg.OutlierAnalysis.CorrelationMaxNormalOccurrence)
	assert.Equal(t, 5, oCfg.OutlierAnalysis.MaxCorrelatedAttributes)
	assert.False(t, oCfg.OutlierAnalysis.PreserveOutliers)
	assert.Equal(t, 2, oCfg.OutlierAnalysis.MaxPreservedOutliers)
	assert.False(t, oCfg.OutlierAnalysis.PreserveOnlyWithCorrelation)
	assert.Equal(t, 0.1, oCfg.OutlierAnalysis.MinOutlierThresholdPercent)

	oCfg.EnableOutlierAnalysis = true
	assert.NoError(t, oCfg.Validate())
}

func TestFactory_CreateTracesProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTraces(
		t.Context(),
		processortest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	assert.NotNil(t, tp)
}
