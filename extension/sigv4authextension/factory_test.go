// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sigv4authextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)

	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, f.CreateDefaultConfig().(*Config), cfg)

	ext, _ := createExtension(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	fext, _ := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	assert.IsType(t, (*sigv4Auth)(nil), ext)
	assert.IsType(t, (*sigv4Auth)(nil), fext)

	actual := fext.(*sigv4Auth)
	expected := ext.(*sigv4Auth)

	assert.Equal(t, expected.cfg, actual.cfg)
	assert.Equal(t, expected.awsSDKInfo, actual.awsSDKInfo)
	assert.NotNil(t, actual.tokenGenerator)
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, &Config{}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}
