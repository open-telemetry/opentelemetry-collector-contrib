// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/metadata"
	xottlfuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"
)

func Test_Standard_ExperimentalFeatureGate(t *testing.T) {
	stableFuncs := StandardFuncs[any]()
	stableConverters := StandardConverters[any]()

	tests := []struct {
		name         string
		got          func() map[string]ottl.Factory[any]
		stable       map[string]ottl.Factory[any]
		experimental []ottl.Factory[any]
	}{
		{
			name:         "StandardFuncs",
			got:          StandardFuncs[any],
			stable:       stableFuncs,
			experimental: xottlfuncs.ExperimentalFuncs[any](),
		},
		{
			name:         "StandardConverters",
			got:          StandardConverters[any],
			stable:       stableConverters,
			experimental: xottlfuncs.ExperimentalConverters[any](),
		},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enabled=%v", tt.name, enabled), func(t *testing.T) {
				defer testutil.SetFeatureGateForTest(t, metadata.PkgOttlFunctionsEnableExperimentalFeatureGate, enabled)()

				got := tt.got()
				for name := range tt.stable {
					assert.Contains(t, got, name)
				}
				for _, f := range tt.experimental {
					_, ok := got[f.Name()]
					assert.Equal(t, enabled, ok, f.Name())
				}

				want := len(tt.stable)
				if enabled {
					want += len(tt.experimental)
				}
				assert.Len(t, got, want)
			})
		}
	}
}
