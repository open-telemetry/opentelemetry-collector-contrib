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

package sapmexporter

import (
	"path"
	"testing"
	"time"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
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

	r0 := cfg.Exporters["sapm"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters["sapm/customname"].(*Config)
	assert.Equal(t, r1,
		&Config{
			ExporterSettings: configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "sapm/customname"},
			Endpoint:         "test-endpoint",
			AccessToken:      "abcd1234",
			NumWorkers:       3,
			MaxConnections:   45,
			AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
				AccessTokenPassthrough: false,
			},
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 10 * time.Second,
			},
			RetrySettings: exporterhelper.RetrySettings{
				Enabled:         true,
				InitialInterval: 10 * time.Second,
				MaxInterval:     1 * time.Minute,
				MaxElapsedTime:  10 * time.Minute,
			},
			QueueSettings: exporterhelper.QueueSettings{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
			},
			Correlation: CorrelationConfig{
				Enabled:             false,
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
			},
		})
}

func TestInvalidConfig(t *testing.T) {
	invalid := Config{
		AccessToken:    "abcd1234",
		NumWorkers:     3,
		MaxConnections: 45,
	}
	noEndpointErr := invalid.validate()
	require.Error(t, noEndpointErr)

	invalid = Config{
		Endpoint:       ":123:456",
		AccessToken:    "abcd1234",
		NumWorkers:     3,
		MaxConnections: 45,
	}
	invalidURLErr := invalid.validate()
	require.Error(t, invalidURLErr)
}
