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

package sumologicextension

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"

	"github.com/SumoLogic/sumologic-otel-collector/pkg/extension/sumologicextension/credentials"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)
	defaultCredsPath := path.Join(homePath, credentials.DefaultCollectorCredentialsDirectory)
	assert.Equal(t, &Config{
		HeartBeatInterval:             DefaultHeartbeatInterval,
		ApiBaseUrl:                    DefaultApiBaseUrl,
		CollectorCredentialsDirectory: defaultCredsPath,
		DiscoverCollectorTags:         true,
		BackOff: backOffConfig{
			InitialInterval: backoff.DefaultInitialInterval,
			MaxInterval:     backoff.DefaultMaxInterval,
			MaxElapsedTime:  backoff.DefaultMaxElapsedTime,
		},
	}, cfg)

	assert.NoError(t, component.ValidateConfig(cfg))

	ccfg := cfg.(*Config)
	ccfg.CollectorName = "test_collector"
	ccfg.Credentials.InstallationToken = "dummy_install_token"

	ext, err := createExtension(context.Background(),
		extension.CreateSettings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		},
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestFactory_CreateExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = "test_collector"
	cfg.Credentials.InstallationToken = "dummy_install_token"

	ext, err := createExtension(context.Background(),
		extension.CreateSettings{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		},
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
