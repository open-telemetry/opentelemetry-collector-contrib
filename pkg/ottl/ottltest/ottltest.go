// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottltest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func Strp(s string) *string {
	return &s
}

func Floatp(f float64) *float64 {
	return &f
}

func Intp(i int64) *int64 {
	return &i
}

func Boolp(b bool) *bool {
	return &b
}

// SetFeatureGateForTest sets the feature gate for the test and returns a function that restores the original value.
func SetFeatureGateForTest(tb testing.TB, gate *featuregate.Gate, enabled bool) func() {
	originalValue := gate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	return func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(gate.ID(), originalValue))
	}
}
