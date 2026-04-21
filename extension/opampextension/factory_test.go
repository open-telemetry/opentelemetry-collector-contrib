// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	assert.Equal(t, createDefaultConfig().(*Config), cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.NoError(t, ext.Shutdown(t.Context()))
}

func TestFactory_Create(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.NoError(t, ext.Shutdown(t.Context()))
}
