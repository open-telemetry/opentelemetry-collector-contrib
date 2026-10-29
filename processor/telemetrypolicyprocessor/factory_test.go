// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetrypolicyprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/telemetrypolicyprocessor/internal/metadata"
)

type wrongConfig struct{}

func (wrongConfig) Validate() error { return nil }

func TestFactoryCreateProcessors(t *testing.T) {
	factory := NewFactory()
	set := processortest.NewNopSettings(metadata.Type)

	validCfg := &Config{
		Providers: []component.ID{component.MustNewID("file_telemetry_policy")},
	}
	wrongTypeCfg := wrongConfig{}

	t.Run("logs", func(t *testing.T) {
		lp, err := factory.CreateLogs(t.Context(), set, validCfg, consumertest.NewNop())
		assert.NoError(t, err)
		assert.NotNil(t, lp)

		_, err = factory.CreateLogs(t.Context(), set, wrongTypeCfg, consumertest.NewNop())
		assert.Error(t, err)
	})

	t.Run("metrics", func(t *testing.T) {
		mp, err := factory.CreateMetrics(t.Context(), set, validCfg, consumertest.NewNop())
		assert.NoError(t, err)
		assert.NotNil(t, mp)

		_, err = factory.CreateMetrics(t.Context(), set, wrongTypeCfg, consumertest.NewNop())
		assert.Error(t, err)
	})

	t.Run("traces", func(t *testing.T) {
		tp, err := factory.CreateTraces(t.Context(), set, validCfg, consumertest.NewNop())
		assert.NoError(t, err)
		assert.NotNil(t, tp)

		_, err = factory.CreateTraces(t.Context(), set, wrongTypeCfg, consumertest.NewNop())
		assert.Error(t, err)
	})
}
