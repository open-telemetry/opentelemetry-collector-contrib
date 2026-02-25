// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/metadata"
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
	assert.Equal(t, metadata.Type, NewFactory().Type())
}

func TestCreateReceiver(t *testing.T) {
	rCfg := createDefaultConfig().(*Config)

	// Fails with bad K8s Config.
	r, err := createLogsReceiver(
		t.Context(), receivertest.NewNopSettings(metadata.Type),
		rCfg, consumertest.NewNop(),
	)
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err)

	// Override for test.
	rCfg.makeClient = func(k8sconfig.APIConfig) (k8s.Interface, error) {
		return fake.NewClientset(), nil
	}
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	rCfg.makeDynamicClient = func(k8sconfig.APIConfig) (dynamic.Interface, error) {
		return dynamicfake.NewSimpleDynamicClient(scheme), nil
	}
	r, err = createLogsReceiver(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		rCfg, consumertest.NewNop(),
	)
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	require.NoError(t, r.Shutdown(t.Context()))
}
