// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operatortest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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
	Expect    any
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
			err = component.UnmarshalConfig(testConfMap, cfg)

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
	Operator operator.Config `mapstructure:"operator"`
}

func newAnyOpConfig(opCfg operator.Builder) *anyOpConfig {
	return &anyOpConfig{
		Operator: operator.Config{Builder: opCfg},
	}
}

func (a *anyOpConfig) Unmarshal(component *confmap.Conf) error {
	return a.Operator.Unmarshal(component)
}
