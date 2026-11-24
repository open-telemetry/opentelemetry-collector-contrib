// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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

	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ext)
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.Equal(t, f.Type(), metadata.Type)
}

func TestFactoryTypeAlias(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, component.MustNewType("basic_auth"), factory.Type())

	for _, typ := range []component.Type{factory.Type(), component.MustNewType("basicauth")} {
		t.Run(typ.String(), func(t *testing.T) {
			for _, mode := range []string{"client", "server"} {
				t.Run(mode, func(t *testing.T) {
					cfg := factory.CreateDefaultConfig().(*Config)
					if mode == "server" {
						cfg.Htpasswd = &HtpasswdSettings{Inline: "username:password"}
					} else {
						cfg.ClientAuth = &ClientAuthSettings{Username: "username", Password: "password"}
					}
					require.NoError(t, cfg.Validate())
					ext, err := factory.Create(t.Context(), extensiontest.NewNopSettings(typ), cfg)
					require.NoError(t, err)
					require.NotNil(t, ext)
					require.NoError(t, ext.Shutdown(t.Context()))
				})
			}
		})
	}
}
