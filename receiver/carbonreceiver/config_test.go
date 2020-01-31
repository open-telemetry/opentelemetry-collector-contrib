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

package carbonreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Receivers[typeStr] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers["carbon"]
	assert.Equal(t, factory.CreateDefaultConfig(), r0)

	r1 := cfg.Receivers["carbon/allsettings"].(*Config)
	assert.Equal(t,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal:  typeStr,
				NameVal:  "carbon/allsettings",
				Endpoint: "localhost:8080",
			},
			Transport:      "udp",
			TCPIdleTimeout: 5 * time.Second,
			Parser: &protocol.Config{
				Type: "delimiter",
				Config: &protocol.DelimiterParser{
					OrDemiliter: "|",
				},
			},
		},
		r1)
}
