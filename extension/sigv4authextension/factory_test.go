// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sigv4authextension

import (
	"context"
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

	ext, _ := createExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	fext, _ := f.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	assert.Equal(t, fext, ext)
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, &Config{}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	ext, err := createExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	assert.Nil(t, err)
	assert.NotNil(t, ext)

}
