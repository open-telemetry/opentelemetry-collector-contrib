// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	expected := &Config{}
	actual := createDefaultConfig()
	assert.Equal(t, expected, createDefaultConfig())
	assert.NoError(t, componenttest.CheckConfigStruct(actual))
}

func TestCreateExtension_ValidConfig(t *testing.T) {
	cfg := &Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	}

	ext, err := createExtension(context.Background(), extensiontest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.Equal(t, f.Type(), metadata.Type)
}
