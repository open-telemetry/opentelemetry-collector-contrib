// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
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

	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)

	lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, lp)

	pp, err := factory.(xprocessor.Factory).CreateProfiles(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
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

	_, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)

	_, err = factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)

	_, err = factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)

	_, err = factory.(xprocessor.Factory).CreateProfiles(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, nil)
	assert.Error(t, err)
}
