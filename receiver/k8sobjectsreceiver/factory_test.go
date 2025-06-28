// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver/internal/metadata"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig()
	rCfg, ok := cfg.(*Config)
	require.True(t, ok)

	assert.Equal(t, &Config{
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
		ErrorMode: PropagateError,
	}, rCfg)
}

func TestFactoryType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, metadata.Type, NewFactory().Type())
}

func TestCreateReceiver(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)

	// Fails with bad K8s Config.
	r, err := createLogsReceiver(
		context.Background(), receivertest.NewNopSettings(metadata.Type),
		rCfg, consumertest.NewNop(),
	)
	assert.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)

	// Override for test.
	rCfg.makeDynamicClient = newMockDynamicClient().getMockDynamicClient

	r, err = createLogsReceiver(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		rCfg, consumertest.NewNop(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}
