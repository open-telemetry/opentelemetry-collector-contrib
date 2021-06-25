// Copyright  OpenTelemetry Authors
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

package observiqexporter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		testName    string
		input       Config
		shouldError bool
	}{
		{
			testName: "Valid config passes",
			input: Config{
				APIKey:   "11111111-2222-3333-4444-555555555555",
				Endpoint: defaultEndpoint,
			},
			shouldError: false,
		},
		{
			testName: "Empty APIKey fails",
			input: Config{
				APIKey:   "",
				Endpoint: defaultEndpoint,
			},
			shouldError: true,
		},
		{
			testName: "Empty Endpoint fails",
			input: Config{
				APIKey:   "11111111-2222-3333-4444-555555555555",
				Endpoint: "",
			},
			shouldError: true,
		},
		{
			testName: "Non-url endpoint fails",
			input: Config{
				APIKey:   "11111111-2222-3333-4444-555555555555",
				Endpoint: "cache_object:foo/bar",
			},
			shouldError: true,
		},
		{
			testName: "Non http or https url fails",
			input: Config{
				APIKey:   "11111111-2222-3333-4444-555555555555",
				Endpoint: "ftp://app.com",
			},
			shouldError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			err := testCase.input.validateConfig()
			if testCase.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, len(cfg.Exporters), 2)

	// Loaded config should be equal to default config (with APIKey filled in)
	defaultCfg := factory.CreateDefaultConfig()
	defaultCfg.(*Config).APIKey = "11111111-2222-3333-4444-555555555555"
	r0 := cfg.Exporters[config.NewID(typeStr)]
	require.Equal(t, r0, defaultCfg)

	r1 := cfg.Exporters[config.NewIDWithName(typeStr, "customname")].(*Config)
	require.Equal(t, r1, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewIDWithName(typeStr, "customname")),
		APIKey:           "11111111-2222-3333-4444-555555555555",
		Endpoint:         "https://sometest.endpoint",
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 10 * time.Second,
		},
		AgentID:   "08e097a6-8580-43f6-b4f5-9d3b4eb2d962",
		AgentName: "otel-collector-1",
		TLSSetting: configtls.TLSClientSetting{
			TLSSetting: configtls.TLSSetting{
				CAFile:   "",
				CertFile: "",
				KeyFile:  "",
			},
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 2,
			QueueSize:    10,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         true,
			InitialInterval: 10 * time.Second,
			MaxInterval:     60 * time.Second,
			MaxElapsedTime:  10 * time.Minute,
		},
	})
}
