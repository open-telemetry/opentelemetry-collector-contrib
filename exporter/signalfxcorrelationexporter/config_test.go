// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signalfxcorrelationexporter

import (
	"path"
	"testing"
	"time"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	facotries, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	facotries.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), facotries,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters["signalfx_correlation"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters["signalfx_correlation/configured"].(*Config)
	assert.Equal(t, r1,
		&Config{
			ExporterSettings:    configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "signalfx_correlation/configured"},
			HTTPClientSettings:  confighttp.HTTPClientSettings{Endpoint: "https://api.signalfx.com", Timeout: 10 * time.Second},
			AccessToken:         "abcd1234",
			StaleServiceTimeout: 5 * time.Minute,
			SyncAttributes: map[string]string{
				"k8s.pod.uid":  "k8s.pod.uid",
				"container.id": "container.id",
			},
			Config: correlations.Config{
				MaxRequests:     20,
				MaxBuffered:     10_000,
				MaxRetries:      2,
				LogUpdates:      false,
				RetryDelay:      30 * time.Second,
				CleanupInterval: 1 * time.Minute,
			},
			HostTranslations: map[string]string{
				"host.name": "host",
			},
		})
}

func TestInvalidConfig(t *testing.T) {
	invalid := Config{
		AccessToken: "abcd1234",
	}
	noEndpointErr := invalid.validate()
	require.Error(t, noEndpointErr)

	invalid = Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ":123:456"},
		AccessToken:        "abcd1234",
	}
	invalidURLErr := invalid.validate()
	require.Error(t, invalidURLErr)
}
