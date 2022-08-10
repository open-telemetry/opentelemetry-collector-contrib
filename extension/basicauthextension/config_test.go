// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package basicauthextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "valid_config.yml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	ext0 := cfg.Extensions[config.NewComponentIDWithName(typeStr, "server")]
	assert.Equal(t, &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "server")),
		Htpasswd: &HtpasswdSettings{
			Inline: "username1:password1\nusername2:password2\n",
		},
	}, ext0)

	ext1 := cfg.Extensions[config.NewComponentIDWithName(typeStr, "client")]
	assert.Equal(t,
		&Config{
			ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "client")),
			ClientAuth: &ClientAuthSettings{
				Username: "username",
				Password: "password",
			},
		},
		ext1)

	assert.Equal(t, 2, len(cfg.Service.Extensions))
	assert.Equal(t, config.NewComponentIDWithName(typeStr, "client"), cfg.Service.Extensions[0])
	assert.Equal(t, config.NewComponentIDWithName(typeStr, "server"), cfg.Service.Extensions[1])
}

func TestLoadConfigError(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	t.Run("invalid config both present", func(t *testing.T) {
		_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_both.yml"), factories)
		assert.Error(t, err)
	})
	t.Run("invalid config none present", func(t *testing.T) {
		_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config_none.yml"), factories)
		assert.Error(t, err)
	})

}
