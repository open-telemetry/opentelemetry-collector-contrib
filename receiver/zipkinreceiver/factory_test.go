// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/coredistro"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig()

	tReceiver, err := createTracesReceiver(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	tReceiver, err = createTracesReceiver(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")
}

func TestCoreDistroRemovalWarning(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	set := receivertest.NewNopSettings(metadata.Type)
	set.BuildInfo = component.BuildInfo{Command: "otelcol", Description: "OpenTelemetry Collector"}
	set.Logger = zap.New(core)

	_, err := NewFactory().CreateTraces(t.Context(), set, createDefaultConfig(), consumertest.NewNop())
	require.NoError(t, err)
	assert.Equal(t, 1, logs.FilterMessageSnippet(coredistro.RemovalWarning).Len())
}
