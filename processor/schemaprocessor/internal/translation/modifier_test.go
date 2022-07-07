// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
)

func newMapFromRaw(tb testing.TB, details map[string]interface{}) pcommon.Map {
	m := pcommon.NewMap()
	require.NoError(tb, m.FromRaw(details), "Must not error when creating new map from raw")
	return m
}

func TestModifyingAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		mod      Modifier
		in       pcommon.Map
		expect   pcommon.Map
	}{
		{
			scenario: "No update",
			mod:      noModify{},
			in: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
		},
		{
			scenario: "No update",
			mod:      newModify(),
			in: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
		},
		{
			scenario: "modified attributes",
			mod: &modify{
				attrs: map[string]string{
					"define":    "label",
					"container": "compute",
				},
				appliesTo: make(map[string]struct{}),
				names:     make(map[string]string),
			},
			in: newMapFromRaw(t, map[string]interface{}{
				"define":    "monkey",
				"update":    "none",
				"container": 1337,
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"label":   "monkey",
				"update":  "none",
				"compute": 1337,
			}),
		},
		{
			scenario: "many modifications",
			mod: &modifications{
				newModify(),
				{attrs: map[string]string{"auto.value": "auto.instrument"}, appliesTo: make(map[string]struct{}), names: make(map[string]string)},
				{attrs: map[string]string{"detected": "discovered"}, appliesTo: make(map[string]struct{}), names: make(map[string]string)},
				newModify(),
			},
			in: newMapFromRaw(t, map[string]interface{}{
				"auto.value": "opentelemetry",
				"detected":   time.Unix(100, 0).Unix(),
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"auto.instrument": "opentelemetry",
				"discovered":      time.Unix(100, 0).Unix(),
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			attrs := pcommon.NewMap()
			tc.in.CopyTo(attrs)

			tc.mod.UpdateAttrs(attrs)
			assert.EqualValues(t, tc.expect.AsRaw(), attrs.AsRaw(), "Must match the expected value once updated")

			tc.mod.RevertAttrs(attrs)
			assert.EqualValues(t, tc.in.AsRaw(), attrs.AsRaw(), "Must match the expected values once reverted")
		})
	}
}

func TestModiyingAttributesIf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		name     string
		mod      Modifier
		in       pcommon.Map
		expect   pcommon.Map
	}{
		{
			scenario: "no update",
			name:     "operation.name",
			mod:      noModify{},
			in: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
		},
		{
			scenario: "no update",
			name:     "operation.name",
			mod:      newModify(),
			in: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
		},
		{
			scenario: "no update when doesn't match applies to",
			name:     "operation.name",
			mod: &modify{
				names: map[string]string{},
				appliesTo: map[string]struct{}{
					"operation.database": {},
				},
				attrs: map[string]string{
					"define": "shark",
					"update": "shark",
				},
			},
			in: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
		},
		{
			scenario: "update when name matches applies to",
			name:     "operation.name",
			mod: &modify{
				names: map[string]string{},
				appliesTo: map[string]struct{}{
					"operation.name": {},
				},
				attrs: map[string]string{
					"define": "label",
					"update": "updated.at",
				},
			},
			in: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"label":      "robot",
				"updated.at": "none",
			}),
		},
		{
			scenario: "update when name matches applies to",
			name:     "operation.name",
			mod: modifications{
				{
					names: map[string]string{},
					appliesTo: map[string]struct{}{
						"operation.name": {},
					},
					attrs: map[string]string{
						"define": "label",
						"update": "updated.at",
					},
				},
				{
					names: map[string]string{},
					appliesTo: map[string]struct{}{
						"memory.usage": {},
					},
					attrs: map[string]string{
						"static": "shark",
					},
				},
			},
			in: newMapFromRaw(t, map[string]interface{}{
				"define": "robot",
				"update": "none",
				"static": "yes",
			}),
			expect: newMapFromRaw(t, map[string]interface{}{
				"label":      "robot",
				"updated.at": "none",
				"static":     "yes",
			}),
		},
	}

	for _, tc := range tests {
		attrs := pcommon.NewMap()
		tc.in.CopyTo(attrs)

		tc.mod.UpdateAttrsIf(tc.name, attrs)
		assert.EqualValues(t, tc.expect.AsRaw(), attrs.AsRaw(), "Must match the expected values")

		tc.mod.RevertAttrsIf(tc.name, attrs)
		assert.EqualValues(t, tc.in.AsRaw(), attrs.AsRaw(), "Must match the expected values")
	}
}

func TestModifyingSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		mod      Modifier
		sig      alias.Signal
		name     string
	}{
		{
			scenario: "no changes",
			mod:      noModify{},
			sig: func() alias.Signal {
				sig := ptrace.NewScopeSpans().Scope()
				sig.SetName("GET /ingress")
				return sig
			}(),
			name: "GET /ingress",
		},
		{
			scenario: "no changes",
			mod:      noModify{},
			sig: func() alias.Signal {
				sig := ptrace.NewScopeSpans().Scope()
				sig.SetName("GET /ingress")
				return sig
			}(),
			name: "GET /ingress",
		},
		{
			scenario: "updated name",
			mod: &modify{
				names: map[string]string{
					"GET /ingress": "GET /v1/api/ingress",
				},
			},
			sig: func() alias.Signal {
				sig := ptrace.NewScopeSpans().Scope()
				sig.SetName("GET /ingress")
				return sig
			}(),
			name: "GET /v1/api/ingress",
		},
		{
			scenario: "updated name with slice",
			mod: modifications{
				&modify{
					names: map[string]string{
						"GET /ingress": "GET /v1/api/ingress",
					},
				},
			},
			sig: func() alias.Signal {
				sig := ptrace.NewScopeSpans().Scope()
				sig.SetName("GET /ingress")
				return sig
			}(),
			name: "GET /v1/api/ingress",
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			name := tc.sig.Name()

			tc.mod.UpdateSignal(tc.sig)
			assert.Equal(t, tc.name, tc.sig.Name(), "Must match the expected name change after update")

			tc.mod.RevertSignal(tc.sig)
			assert.Equal(t, name, tc.sig.Name(), "Must match the expected name change after revert")
		})
	}
}
