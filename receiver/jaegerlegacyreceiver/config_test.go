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

package jaegerlegacyreceiver

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configprotocol"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// The receiver `jaeger_legacy/disabled` and `jaeger_legacy` don't count because disabled receivers
	// are excluded from the final list.
	assert.Equal(t, 2, len(cfg.Receivers))

	r1 := cfg.Receivers["jaeger_legacy/customname"].(*Config)
	assert.Equal(t, r1,
		&Config{
			TypeVal: configmodels.Type(typeStr),
			NameVal: "jaeger_legacy/customname",
			Protocols: map[string]*configprotocol.ProtocolServerSettings{
				"thrift_tchannel": {
					Endpoint: "0.0.0.0:123",
				},
			},
		})

	rDefaults := cfg.Receivers["jaeger_legacy/defaults"].(*Config)
	assert.Equal(t, rDefaults,
		&Config{
			TypeVal: configmodels.Type(typeStr),
			NameVal: "jaeger_legacy/defaults",
			Protocols: map[string]*configprotocol.ProtocolServerSettings{
				"thrift_tchannel": {
					Endpoint: defaultTChannelBindEndpoint,
				},
			},
		})
}

func TestFailedLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Receivers[configmodels.Type(typeStr)] = factory
	_, err = config.LoadConfigFile(t, path.Join(".", "testdata", "bad_proto_config.yaml"), factories)
	assert.EqualError(t, err, `error reading settings for receiver type "jaeger_legacy": unknown Jaeger Legacy protocol badproto`)

	_, err = config.LoadConfigFile(t, path.Join(".", "testdata", "bad_no_proto_config.yaml"), factories)
	assert.EqualError(t, err, `error reading settings for receiver type "jaeger_legacy": must specify at least one protocol when using the Jaeger Legacy receiver`)

	_, err = config.LoadConfigFile(t, path.Join(".", "testdata", "bad_empty_config.yaml"), factories)
	assert.EqualError(t, err, `error reading settings for receiver type "jaeger_legacy": jaeger legacy receiver config is empty`)
}
