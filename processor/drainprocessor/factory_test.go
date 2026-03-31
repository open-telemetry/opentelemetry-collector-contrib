// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	require.NoError(t, componenttest.CheckConfigStruct(cfg))

	dc := cfg.(*Config)
	assert.Equal(t, 4, dc.LogClusterDepth)
	assert.InDelta(t, 0.4, dc.SimThreshold, 1e-9)
	assert.Equal(t, 100, dc.MaxChildren)
	assert.Equal(t, 0, dc.MaxClusters)
	assert.Equal(t, "log.record.template", dc.TemplateAttribute)
	assert.Equal(t, warmupModePassthrough, dc.WarmupMode)
}

func TestCreateLogsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := processortest.NewNopSettings(metadata.Type)

	lp, err := factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, lp)

	require.NoError(t, lp.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, lp.Shutdown(t.Context()))
}
