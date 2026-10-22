// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinv2

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func TestValidateFeatureGates(t *testing.T) {
	const (
		dontEmitV0HTTP  = "pkg.translator.zipkin.DontEmitV0HttpConventions"
		emitV1HTTP      = "pkg.translator.zipkin.EmitV1HttpConventions"
		dontEmitV0Scope = "pkg.translator.zipkin.DontEmitV0ScopeConventions"
		emitV1Scope     = "pkg.translator.zipkin.EmitV1ScopeConventions"
	)
	allGates := []string{
		dontEmitV0HTTP, emitV1HTTP,
		dontEmitV0Scope, emitV1Scope,
	}

	tests := []struct {
		name string
		// disable lists the gates set to false for the case; every other gate is enabled.
		disable []string
		wantErr string
	}{
		{
			name:    "all enabled (beta defaults)",
			disable: nil,
			wantErr: "",
		},
		{
			name:    "http DontEmitV0 without EmitV1",
			disable: []string{emitV1HTTP},
			wantErr: "pkg.translator.zipkin.DontEmitV0HttpConventions cannot be enabled without enabling pkg.translator.zipkin.EmitV1HttpConventions",
		},
		{
			name:    "scope DontEmitV0 without EmitV1",
			disable: []string{emitV1Scope},
			wantErr: "pkg.translator.zipkin.DontEmitV0ScopeConventions cannot be enabled without enabling pkg.translator.zipkin.EmitV1ScopeConventions",
		},
		{
			name:    "http fully disabled is valid",
			disable: []string{dontEmitV0HTTP, emitV1HTTP},
			wantErr: "",
		},
		{
			name:    "scope emitting both v0 and v1 is valid",
			disable: []string{dontEmitV0Scope},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disabled := make(map[string]bool, len(tt.disable))
			for _, g := range tt.disable {
				disabled[g] = true
			}
			for _, g := range allGates {
				require.NoError(t, featuregate.GlobalRegistry().Set(g, !disabled[g]))
			}
			t.Cleanup(func() {
				for _, g := range allGates {
					require.NoError(t, featuregate.GlobalRegistry().Set(g, true))
				}
			})

			err := ValidateFeatureGates()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
