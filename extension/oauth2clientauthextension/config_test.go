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

package oauth2clientauthextension

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	expected := factory.CreateDefaultConfig().(*Config)
	expected.ClientSecret = ValueValueFrom{Value: "someclientsecret"}
	expected.ClientID = ValueValueFrom{Value: "someclientid"}
	expected.Scopes = []string{"api.metrics"}
	expected.TokenURL = ValueValueFrom{Value: "https://example.com/oauth2/default/v1/token"}

	ext := cfg.Extensions[config.NewComponentIDWithName(typeStr, "1")]
	assert.Equal(t,
		&Config{
			ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "1")),
			ClientSecret:      ValueValueFrom{Value: "someclientsecret"},
			ClientID:          ValueValueFrom{Value: "someclientid"},
			EndpointParams:    url.Values{"audience": []string{"someaudience"}},
			Scopes:            []string{"api.metrics"},
			TokenURL:          ValueValueFrom{Value: "https://example.com/oauth2/default/v1/token"},
			Timeout:           time.Second,
		},
		ext)

	assert.Equal(t, 2, len(cfg.Service.Extensions))
	assert.Equal(t, config.NewComponentIDWithName(typeStr, "1"), cfg.Service.Extensions[0])
}

func TestConfigTLSSettings(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	ext2 := cfg.Extensions[config.NewComponentIDWithName(typeStr, "withtls")]

	cfg2 := ext2.(*Config)
	assert.Equal(t, cfg2.TLSSetting, configtls.TLSClientSetting{
		TLSSetting: configtls.TLSSetting{
			CAFile:   "cafile",
			CertFile: "certfile",
			KeyFile:  "keyfile",
		},
		Insecure:           true,
		InsecureSkipVerify: false,
		ServerName:         "",
	})
}

func TestLoadConfigError(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	tests := []struct {
		configName  string
		expectedErr error
	}{
		{
			"missingurl",
			errNoTokenURLProvided,
		},
		{
			"missingid",
			errNoClientIDProvided,
		},
		{
			"missingsecret",
			errNoClientSecretProvided,
		},
	}
	for _, tt := range tests {
		factory := NewFactory()
		factories.Extensions[typeStr] = factory
		cfg, _ := servicetest.LoadConfig(filepath.Join("testdata", "config_bad.yaml"), factories)
		extension := cfg.Extensions[config.NewComponentIDWithName(typeStr, tt.configName)]
		verr := extension.Validate()
		require.ErrorIs(t, verr, tt.expectedErr)
	}
}

func TestEnv(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config_env.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	expected := factory.CreateDefaultConfig().(*Config)
	expected.ClientSecret = ValueValueFrom{ValueFrom: ValueFrom{Env: "someclientsecret"}}
	expected.ClientID = ValueValueFrom{ValueFrom: ValueFrom{Env: "someclientid"}}
	expected.Scopes = []string{"api.metrics"}
	expected.TokenURL = ValueValueFrom{ValueFrom: ValueFrom{Env: "https://example.com/oauth2/default/v1/token"}}

	ext := cfg.Extensions[config.NewComponentIDWithName(typeStr, "1")]
	assert.Equal(t,
		&Config{
			ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "1")),
			ClientSecret:      ValueValueFrom{ValueFrom: ValueFrom{Env: "someclientsecret"}},
			ClientID:          ValueValueFrom{ValueFrom: ValueFrom{Env: "someclientid"}},
			EndpointParams:    url.Values{"audience": []string{"someaudience"}},
			Scopes:            []string{"api.metrics"},
			TokenURL:          ValueValueFrom{ValueFrom: ValueFrom{Env: "https://example.com/oauth2/default/v1/token"}},
			Timeout:           time.Second,
		},
		ext)

	assert.Equal(t, 2, len(cfg.Service.Extensions))
	assert.Equal(t, config.NewComponentIDWithName(typeStr, "1"), cfg.Service.Extensions[0])
}

func TestFile(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config_file.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	expected := factory.CreateDefaultConfig().(*Config)
	expected.ClientSecret = ValueValueFrom{ValueFrom: ValueFrom{File: "someclientsecret"}}
	expected.ClientID = ValueValueFrom{ValueFrom: ValueFrom{File: "someclientid"}}
	expected.Scopes = []string{"api.metrics"}
	expected.TokenURL = ValueValueFrom{ValueFrom: ValueFrom{File: "https://example.com/oauth2/default/v1/token"}}

	ext := cfg.Extensions[config.NewComponentIDWithName(typeStr, "1")]
	assert.Equal(t,
		&Config{
			ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "1")),
			ClientSecret:      ValueValueFrom{ValueFrom: ValueFrom{File: "someclientsecret"}},
			ClientID:          ValueValueFrom{ValueFrom: ValueFrom{File: "someclientid"}},
			EndpointParams:    url.Values{"audience": []string{"someaudience"}},
			Scopes:            []string{"api.metrics"},
			TokenURL:          ValueValueFrom{ValueFrom: ValueFrom{File: "https://example.com/oauth2/default/v1/token"}},
			Timeout:           time.Second,
		},
		ext)

	assert.Equal(t, 2, len(cfg.Service.Extensions))
	assert.Equal(t, config.NewComponentIDWithName(typeStr, "1"), cfg.Service.Extensions[0])
}

func TestParse(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	os.Setenv("TEST", "someclientid")
	defer os.Unsetenv("TEST")

	tests := []struct {
		entry    ValueValueFrom
		expected string
	}{
		{
			entry:    ValueValueFrom{ValueFrom: ValueFrom{Env: "TEST"}},
			expected: "someclientid",
		},
		{
			entry:    ValueValueFrom{Value: "someclientid2"},
			expected: "someclientid2",
		},
		{
			entry:    ValueValueFrom{ValueFrom: ValueFrom{File: filepath.Join("testdata", "testfile")}},
			expected: "someclientid3",
		},
	}
	for _, tt := range tests {
		factory := NewFactory()
		factories.Extensions[typeStr] = factory
		cfg := factory.CreateDefaultConfig().(*Config)
		actual, err := cfg.Parse(tt.entry)
		assert.Equal(t, tt.expected, actual)
		assert.Equal(t, nil, err)
	}
}
