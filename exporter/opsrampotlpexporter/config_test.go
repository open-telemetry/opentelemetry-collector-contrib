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

package opsrampotlpexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opsrampotlpexporter/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[metadata.Type] = factory

	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[component.NewID(metadata.Type)]

	// e1 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "2")]
	assert.Equal(t, e,
		&Config{
			TimeoutConfig: exporterhelper.TimeoutConfig{
				Timeout: 10 * time.Second,
			},
			BackOffConfig: configretry.BackOffConfig{
				Enabled:         true,
				InitialInterval: 10 * time.Second,
				MaxInterval:     1 * time.Minute,
				MaxElapsedTime:  10 * time.Minute,
			},
			QueueConfig: exporterhelper.QueueConfig{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
			},
			Security: SecuritySettings{
				ClientID:        "id",
				ClientSecret:    "secret",
				OAuthServiceURL: "url",
			},
			ClientConfig: configgrpc.ClientConfig{
				Headers: map[string]configopaque.String{
					"can you have a . here?": "F0000000-0000-0000-0000-000000000000",
					"header1":                "234",
					"another":                "somevalue",
				},
				Endpoint:    "1.2.3.4:1234",
				Compression: "gzip",
				TLSSetting: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: "/var/lib/mycert.pem",
					},
					Insecure: false,
				},
				Keepalive: &configgrpc.KeepaliveClientConfig{
					Time:                20 * time.Second,
					PermitWithoutStream: true,
					Timeout:             30 * time.Second,
				},
				WriteBufferSize: 512 * 1024,
				BalancerName:    "round_robin",
				Auth:            &configauth.Authentication{AuthenticatorID: component.NewID(component.MustNewType("nop"))},
			},
		})
}
