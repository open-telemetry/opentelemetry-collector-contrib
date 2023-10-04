// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, component.UseLocalHostAsDefaultHostfeatureGate, false)()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	assert.Equal(t, component.Type(metadata.Type), factory.Type())
	awsxrayCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, awsxrayCfg.Endpoint, "0.0.0.0:2000")
}

func TestCreateDefaultConfigLocalHost(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, component.UseLocalHostAsDefaultHostfeatureGate, true)()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	assert.Equal(t, component.Type(metadata.Type), factory.Type())
	awsxrayCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, awsxrayCfg.Endpoint, "localhost:2000")
}

func TestCreateTracesReceiver(t *testing.T) {
	// TODO review if test should succeed on Windows
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	t.Setenv(defaultRegionEnvName, mockRegion)

	factory := NewFactory()
	_, err := factory.CreateTracesReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		factory.CreateDefaultConfig().(*Config),
		consumertest.NewNop(),
	)
	assert.Nil(t, err, "trace receiver can be created")
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		factory.CreateDefaultConfig().(*Config),
		consumertest.NewNop(),
	)
	assert.NotNil(t, err, "a trace receiver factory should not create a metric receiver")
	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
}
