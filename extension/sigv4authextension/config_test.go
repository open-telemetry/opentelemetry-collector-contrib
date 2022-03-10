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

package sigv4authextension

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	awsCredsProvider := mockCredentials()
	awsCreds, _ := (*awsCredsProvider).Retrieve(context.Background())

	t.Setenv("AWS_ACCESS_KEY_ID", awsCreds.AccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", awsCreds.SecretAccessKey)

	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Region = "region"
	expected.Service = "service"
	expected.RoleSessionName = "role_session_name"

	ext := cfg.Extensions[config.NewComponentID(typeStr)]
	// Ensure creds are the same for load config test; tested in extension_test.go
	expected.credsProvider = ext.(*Config).credsProvider
	assert.Equal(t, expected, ext)

	assert.Equal(t, 1, len(cfg.Service.Extensions))
	assert.Equal(t, config.NewComponentID(typeStr), cfg.Service.Extensions[0])
}

func TestLoadConfigError(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	tests := []struct {
		name        string
		expectedErr error
	}{
		{
			"missing_credentials",
			errBadCreds,
		},
	}
	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			factory := NewFactory()
			factories.Extensions[typeStr] = factory
			cfg, _ := servicetest.LoadConfig(path.Join(".", "testdata", "config_bad.yaml"), factories)
			extension := cfg.Extensions[config.NewComponentIDWithName(typeStr, testcase.name)]
			verr := extension.Validate()
			require.ErrorIs(t, verr, testcase.expectedErr)
		})

	}
}
