// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operatortest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// ConfigUnmarshalTest is used for testing golden configs
type ConfigUnmarshalTests struct {
	Factory   operator.Factory
	TestsFile string
	Tests     []ConfigUnmarshalTest
}

// ConfigUnmarshalTest is used for testing golden configs
type ConfigUnmarshalTest struct {
	Name      string
	Expect    any
	ExpectErr bool
}

// Run Unmarshals yaml files and compares them against the expected.
func (c ConfigUnmarshalTests) Run(t *testing.T) {
	require.NotNil(t, c.Factory, "test setup error: operator factory must be specified")

	testConfMaps, err := confmaptest.LoadConf(c.TestsFile)
	require.NoError(t, err)

	for _, tc := range c.Tests {
		t.Run(tc.Name, func(t *testing.T) {
			testConfMap, err := testConfMaps.Sub(tc.Name)
			require.NoError(t, err)
			require.NotZero(t, len(testConfMap.AllKeys()), fmt.Sprintf("config not found: '%s'", tc.Name))

			err = component.UnmarshalConfig(testConfMap, c.Factory.NewDefaultConfig(""))
			if tc.ExpectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				_, ok := operator.LookupFactory(c.Factory.Type())
				require.True(t, ok, "expected factory to be registered")
			}
		})
	}
}
