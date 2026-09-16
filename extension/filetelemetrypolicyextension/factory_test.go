// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filetelemetrypolicyextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/filetelemetrypolicyextension/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, component.MustNewType("file_telemetry_policy"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, &Config{}, cfg)
}

func TestCreateExtension(t *testing.T) {
	cfg := createDefaultConfig()
	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
