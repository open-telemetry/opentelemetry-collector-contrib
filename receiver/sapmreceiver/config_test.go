// Copyright 2019, OpenTelemetry Authors
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

package sapmreceiver

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// The receiver `sapm/disabled` doesn't count because disabled receivers
	// are excluded from the final list.
	assert.Equal(t, len(cfg.Receivers), 4)

	r0 := cfg.Receivers["sapm"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Receivers["sapm/customname"].(*Config)
	assert.Equal(t, r1,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: typeStr,
				NameVal: "sapm/customname",
			},
			Endpoint: "0.0.0.0:7276",
		})

	r2 := cfg.Receivers["sapm/tls"].(*Config)
	assert.Equal(t, r2,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: typeStr,
				NameVal: "sapm/tls",
			},
			Endpoint: ":7276",
			TLSCredentials: &configtls.TLSSetting{
				CertFile: "/test.crt",
				KeyFile:  "/test.key",
			},
		})

	r3 := cfg.Receivers["sapm/passthrough"].(*Config)
	assert.Equal(t, r3,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: typeStr,
				NameVal: "sapm/passthrough",
			},
			Endpoint: ":7276",
			AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
				AccessTokenPassthrough: true,
			},
		})
}
