// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticsearchreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestValidateCredentials(t *testing.T) {
	testCases := []struct {
		desc string
		run  func(t *testing.T)
	}{
		{
			desc: "Password is empty, username specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Username = "user"
				require.ErrorIs(t, cfg.Validate(), errPasswordNotSpecified)
			},
		},
		{
			desc: "Username is empty, password specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Password = "pass"
				require.ErrorIs(t, cfg.Validate(), errUsernameNotSpecified)
			},
		},
		{
			desc: "Username and password are both specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Username = "user"
				cfg.Password = "pass"
				require.NoError(t, cfg.Validate())
			},
		},
		{
			desc: "Username and password are both not specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				require.NoError(t, cfg.Validate())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, testCase.run)
	}
}

func TestValidateEndpoint(t *testing.T) {
	testCases := []struct {
		desc           string
		rawURL         string
		expectedErr    error
		expectedErrStr string
	}{
		{
			desc:   "Default endpoint",
			rawURL: defaultEndpoint,
		},
		{
			desc:        "Empty endpoint",
			rawURL:      "",
			expectedErr: errEmptyEndpoint,
		},
		{
			desc:        "Endpoint with no scheme",
			rawURL:      "localhost",
			expectedErr: errEndpointBadScheme,
		},
		{
			desc:        "Endpoint with unusable scheme",
			rawURL:      "tcp://192.168.1.0",
			expectedErr: errEndpointBadScheme,
		},
		{
			desc:           "URL with control characters",
			rawURL:         "http://\x00",
			expectedErrStr: "invalid endpoint",
		},
		{
			desc:   "Https url",
			rawURL: "https://example.com",
		},
		{
			desc:   "IP + port URL",
			rawURL: "https://192.168.1.1:9200",
		},
	}
	for i := range testCases {
		// Explicitly capture the testCase in this scope instead of using a loop variable;
		// The loop variable will mutate, and all tests will run with the parameters of the last case,
		// if we don't do this
		testCase := testCases[i]
		t.Run(testCase.desc, func(t *testing.T) {
			t.Parallel()

			cfg := NewFactory().CreateDefaultConfig().(*Config)
			cfg.Endpoint = testCase.rawURL

			err := cfg.Validate()

			switch {
			case testCase.expectedErr != nil:
				require.ErrorIs(t, err, testCase.expectedErr)
			case testCase.expectedErrStr != "":
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.expectedErrStr)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, len(cfg.Receivers), 2)
	defaultRecvID := config.NewComponentIDWithName(typeStr, "defaults")

	defaultCfg := factory.CreateDefaultConfig().(*Config)
	defaultCfg.ReceiverSettings.SetIDName(defaultRecvID.Name())
	defaultReceiver := cfg.Receivers[defaultRecvID]
	require.Equal(t, defaultCfg, defaultReceiver)

	advancedRecv := cfg.Receivers[config.NewComponentID(typeStr)]
	expectedAdvancedRecv := factory.CreateDefaultConfig().(*Config)

	expectedAdvancedRecv.Metrics.ElasticsearchNodeFsDiskAvailable.Enabled = false
	expectedAdvancedRecv.Nodes = []string{"_local"}
	expectedAdvancedRecv.SkipClusterMetrics = true
	expectedAdvancedRecv.Username = "otel"
	expectedAdvancedRecv.Password = "password"
	expectedAdvancedRecv.Endpoint = "http://example.com:9200"
	expectedAdvancedRecv.ScraperControllerSettings.CollectionInterval = 2 * time.Minute

	require.Equal(t, expectedAdvancedRecv, advancedRecv)
}
