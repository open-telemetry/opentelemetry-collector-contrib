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

package cloudfoundryreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

// todo implement - implement negative tests in stage 2 PR
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Len(t, cfg.Receivers, 2)

	r0 := cfg.Receivers[config.NewComponentID(typeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), r0)

	r1 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "one")].(*Config)
	assert.Equal(t,
		&Config{
			ReceiverSettings:        config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "one")),
			RLPGatewayURL:           "https://log-stream.sys.example.internal",
			RLPGatewaySkipTLSVerify: true,
			RLPGatewayShardID:       "otel-test",
			UAAUrl:                  "https://uaa.sys.example.internal",
			UAASkipTLSVerify:        true,
			UAAUsername:             "admin",
			UAAPassword:             "test",
			HTTPTimeout:             time.Second * 20,
		}, r1)
}
