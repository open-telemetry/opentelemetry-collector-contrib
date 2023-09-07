// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	rCfg, ok := cfg.(*Config)
	require.True(t, ok)

	assert.Equal(t, &Config{
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}, rCfg)
}

func TestFactoryType(t *testing.T) {
	assert.Equal(t, component.Type("k8s_events"), NewFactory().Type())
}

func TestCreateReceiver(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)

	// Fails with bad K8s Config.
	r, err := createLogsReceiver(
		context.Background(), receivertest.NewNopCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	assert.Error(t, err)
	assert.Nil(t, r)

	// Override for test.
	rCfg.makeClient = func(apiConf k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewSimpleClientset(), nil
	}
	r, err = createLogsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}
