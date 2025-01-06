// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount}, cfg.APIConfig)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_Create(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	ext, err := factory.Create(context.Background(), extensiontest.NewNopSettings(), cfg)
	require.Error(t, err)
	require.Nil(t, ext)
}
