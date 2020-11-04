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

package splunkhecreceiver

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 3)

	r0 := cfg.Receivers["splunk_hec"]
	assert.Equal(t, r0, createDefaultConfig())

	r1 := cfg.Receivers["splunk_hec/allsettings"].(*Config)
	assert.Equal(t, r1,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: typeStr,
				NameVal: "splunk_hec/allsettings",
			},
			HTTPServerSettings: confighttp.HTTPServerSettings{
				Endpoint: "localhost:8088",
			},
			AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
				AccessTokenPassthrough: true,
			},
		})

	r2 := cfg.Receivers["splunk_hec/tls"].(*Config)
	assert.Equal(t, r2,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: typeStr,
				NameVal: "splunk_hec/tls",
			},
			HTTPServerSettings: confighttp.HTTPServerSettings{
				Endpoint: ":8088",
				TLSSetting: &configtls.TLSServerSetting{
					TLSSetting: configtls.TLSSetting{
						CertFile: "/test.crt",
						KeyFile:  "/test.key",
					},
				},
			},
			AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
				AccessTokenPassthrough: false,
			},
		})
}
