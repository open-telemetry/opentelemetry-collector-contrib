// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestCreateDefaultConfig(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, component.UseLocalHostAsDefaultHostfeatureGate, false)()
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	awsfirehoseCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, awsfirehoseCfg.Endpoint, "0.0.0.0:4433")
}

func TestCreateDefaultConfigLocalHost(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, component.UseLocalHostAsDefaultHostfeatureGate, true)()
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	awsxrayCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, awsxrayCfg.Endpoint, "localhost:4433")
}

func TestCreateMetricsReceiver(t *testing.T) {
	r, err := createMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)
}

func TestValidateRecordType(t *testing.T) {
	require.NoError(t, validateRecordType(defaultRecordType))
	require.Error(t, validateRecordType("nop"))
}
