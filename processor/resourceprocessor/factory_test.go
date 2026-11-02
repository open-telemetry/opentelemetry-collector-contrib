// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/coredistro"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		AttributesActions: []attraction.ActionKeyValue{
			{Key: "cloud.availability_zone", Value: "zone-1", Action: attraction.UPSERT},
		},
	}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	mp, err := factory.CreateMetrics(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)

	lp, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, lp)

	pp, err := factory.(xprocessor.Factory).CreateProfiles(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, pp)
}

func TestInvalidAttributeActions(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		AttributesActions: []attraction.ActionKeyValue{
			{Key: "k", Value: "v", Action: "invalid-action"},
		},
	}

	_, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)

	_, err = factory.CreateMetrics(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)

	_, err = factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)

	_, err = factory.(xprocessor.Factory).CreateProfiles(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)
}

func TestCoreDistroRemovalWarning(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	set := processortest.NewNopSettings(metadata.Type)
	set.BuildInfo = component.BuildInfo{Command: "otelcol", Description: "OpenTelemetry Collector"}
	set.Logger = zap.New(core)

	factory := NewFactory()
	cfg := &Config{AttributesActions: []attraction.ActionKeyValue{{Key: "k", Value: "v", Action: attraction.UPSERT}}}

	_, err := factory.CreateTraces(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	_, err = factory.CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	_, err = factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	_, err = factory.(xprocessor.Factory).CreateProfiles(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.Equal(t, 4, logs.FilterMessageSnippet(coredistro.RemovalWarning).Len())
}
