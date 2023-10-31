// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
			sig: NewSignalNameChange(
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
			sig: NewSignalNameChange(
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
			sig: NewSignalNameChange(map[string]string{
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
			sig: NewSignalNameChange(
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
			sig: NewSignalNameChange(
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
			sig: NewSignalNameChange(map[string]string{
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
