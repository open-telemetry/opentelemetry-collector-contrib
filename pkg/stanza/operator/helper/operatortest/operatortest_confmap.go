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

package operatortest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// ConfigUnmarshalTest is used for testing golden configs
type ConfigUnmarshalTests struct {
	DefaultConfig operator.Builder
	TestsFile     string
	Tests         []ConfigUnmarshalTest
}

// ConfigUnmarshalTest is used for testing golden configs
type ConfigUnmarshalTest struct {
	Name      string
	Expect    interface{}
	ExpectErr bool
}

// Run Unmarshals yaml files and compares them against the expected.
func (c ConfigUnmarshalTests) Run(t *testing.T) {
	testConfMaps, err := confmaptest.LoadConf(c.TestsFile)
	require.NoError(t, err)

	for _, tc := range c.Tests {
		t.Run(tc.Name, func(t *testing.T) {
			testConfMap, err := testConfMaps.Sub(tc.Name)
			require.NoError(t, err)
			require.NotZero(t, len(testConfMap.AllKeys()), fmt.Sprintf("config not found: '%s'", tc.Name))

			cfg := newAnyOpConfig(c.DefaultConfig)
			err = config.UnmarshalReceiver(testConfMap, cfg)

			if tc.ExpectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.Expect, cfg.Operator.Builder)
			}
		})
	}
}

type anyOpConfig struct {
	config.ReceiverSettings `mapstructure:",squash"`
	Operator                operator.Config `mapstructure:"operator"`
}

func newAnyOpConfig(opCfg operator.Builder) *anyOpConfig {
	return &anyOpConfig{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID("any_op")),
		Operator:         operator.Config{Builder: opCfg},
	}
}

func (a *anyOpConfig) Unmarshal(component *confmap.Conf) error {
	return a.Operator.Unmarshal(component)
}
