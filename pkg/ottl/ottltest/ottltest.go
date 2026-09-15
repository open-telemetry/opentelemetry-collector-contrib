// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottltest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

//go:fix inline
func Strp(s string) *string {
	return new(s)
}

//go:fix inline
func Floatp(f float64) *float64 {
	return new(f)
}

//go:fix inline
func Intp(i int64) *int64 {
	return new(i)
}

//go:fix inline
func Boolp(b bool) *bool {
	return new(b)
}

// SetFeatureGateForTest sets the feature gate for the test and returns a function that restores the original value.
func SetFeatureGateForTest(tb testing.TB, gate *featuregate.Gate, enabled bool) func() {
	originalValue := gate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	return func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(gate.ID(), originalValue))
	}
}
