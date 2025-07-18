// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default configuration")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()

	cfg := &Config{
		Lookups: []lookupConfig{
			{
				Type:             resolve,
				Context:          resource,
				SourceAttributes: []string{string(semconv.SourceAddressKey)},
				TargetAttribute:  sourceIPKey,
			},
			{
				Type:             reverse,
				Context:          resource,
				SourceAttributes: []string{sourceIPKey},
				TargetAttribute:  string(semconv.SourceAddressKey),
			},
		},
	}

	params := processortest.NewNopSettings(metadata.Type)

	tp, err := factory.CreateTraces(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	mp, err := factory.CreateMetrics(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)

	lp, err := factory.CreateLogs(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, lp)
	assert.NoError(t, err)

	tp, err = factory.CreateTraces(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	mp, err = factory.CreateMetrics(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)

	lp, err = factory.CreateLogs(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, lp)
	assert.NoError(t, err)
}
