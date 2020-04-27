// Copyright 2020, OpenTelemetry Authors
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

package simpleprometheusreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	receiverType := "prometheus_simple"
	factories.Receivers[receiverType] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 3)

	r1 := cfg.Receivers[receiverType]
	assert.Equal(t, r1, factory.CreateDefaultConfig())

	r2 := cfg.Receivers["prometheus_simple/all_settings"].(*Config)
	assert.Equal(t, r2,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal:  receiverType,
				NameVal:  "prometheus_simple/all_settings",
				Endpoint: "localhost:1234",
			},
			TLSConfig: &tlsConfig{
				CAFile:             "path",
				CertFile:           "path",
				KeyFile:            "path",
				InsecureSkipVerify: true,
			},
			CollectionInterval: 30 * time.Second,
			MetricsPath:        "/v2/metrics",
			UseServiceAccount:  true,
		})

	r3 := cfg.Receivers["prometheus_simple/partial_settings"].(*Config)
	assert.Equal(t, r3,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal:  receiverType,
				NameVal:  "prometheus_simple/partial_settings",
				Endpoint: "localhost:1234",
			},
			CollectionInterval: 30 * time.Second,
			MetricsPath:        "/metrics",
		})
}
