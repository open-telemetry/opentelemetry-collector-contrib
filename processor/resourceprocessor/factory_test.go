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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
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

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)
}

func TestInvalidAttributeActions(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		AttributesActions: []attraction.ActionKeyValue{
			{Key: "k", Value: "v", Action: "invalid-action"},
		},
	}

	_, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, nil)
	assert.Error(t, err)

	_, err = factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, nil)
	assert.Error(t, err)
}
