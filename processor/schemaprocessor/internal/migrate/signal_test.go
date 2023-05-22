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

package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
)

func TestSignalApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		sig    *SignalNameChange
		val    alias.NamedSignal
		expect string
	}{
		{
			name: "No changes",
			sig: NewSignal(
				map[string]string{},
			),
			val: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("system.uptime")
				return m
			}(),
			expect: "system.uptime",
		},
		{
			name: "No matched changes",
			sig: NewSignal(
				map[string]string{
					"system.cpu.usage": "cpu.usage",
				},
			),
			val: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("system.uptime")
				return m
			}(),
			expect: "system.uptime",
		},
		{
			name: "Matched changes",
			sig: NewSignal(map[string]string{
				"system.uptime": "instance.uptime",
			}),
			val: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("system.uptime")
				return m
			}(),
			expect: "instance.uptime",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.sig.Apply(tc.val)
			assert.Equal(t, tc.expect, tc.val.Name(), "Must match expected name")
		})
	}
}

func TestSignalRollback(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		sig    *SignalNameChange
		val    alias.NamedSignal
		expect string
	}{
		{
			name: "No changes",
			sig: NewSignal(
				map[string]string{},
			),
			val: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("system.uptime")
				return m
			}(),
			expect: "system.uptime",
		},
		{
			name: "No matched changes",
			sig: NewSignal(
				map[string]string{
					"system.cpu.usage": "cpu.usage",
				},
			),
			val: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("system.uptime")
				return m
			}(),
			expect: "system.uptime",
		},
		{
			name: "Matched changes",
			sig: NewSignal(map[string]string{
				"system.uptime": "instance.uptime",
			}),
			val: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("instance.uptime")
				return m
			}(),
			expect: "system.uptime",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.sig.Rollback(tc.val)
			assert.Equal(t, tc.expect, tc.val.Name(), "Must match expected name")
		})
	}
}
