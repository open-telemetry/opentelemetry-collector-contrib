// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bearertokenauthextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{Scheme: defaultScheme}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.BearerToken = "somerandometoken"
	ext, err := createExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
