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
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 4)

	r1 := cfg.Receivers[config.NewID(typeStr)]
	assert.Equal(t, r1, factory.CreateDefaultConfig())

	r2 := cfg.Receivers[config.NewIDWithName(typeStr, "all_settings")].(*Config)
	assert.Equal(t, r2,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "all_settings")),
			TCPAddr: confignet.TCPAddr{
				Endpoint: "localhost:1234",
			},
			httpConfig: httpConfig{
				TLSEnabled: true,
				TLSConfig: tlsConfig{
					CAFile:             "path",
					CertFile:           "path",
					KeyFile:            "path",
					InsecureSkipVerify: true,
				},
			},
			CollectionInterval: 30 * time.Second,
			MetricsPath:        "/v2/metrics",
			Params:             url.Values{"columns": []string{"name", "messages"}, "key": []string{"foo", "bar"}},
			UseServiceAccount:  true,
		})

	r3 := cfg.Receivers[config.NewIDWithName(typeStr, "partial_settings")].(*Config)
	assert.Equal(t, r3,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "partial_settings")),
			TCPAddr: confignet.TCPAddr{
				Endpoint: "localhost:1234",
			},
			CollectionInterval: 30 * time.Second,
			MetricsPath:        "/metrics",
		})

	r4 := cfg.Receivers[config.NewIDWithName(typeStr, "partial_tls_settings")].(*Config)
	assert.Equal(t, r4,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(typeStr, "partial_tls_settings")),
			TCPAddr: confignet.TCPAddr{
				Endpoint: "localhost:1234",
			},
			httpConfig: httpConfig{
				TLSEnabled: true,
			},
			CollectionInterval: 30 * time.Second,
			MetricsPath:        "/metrics",
		})
}
