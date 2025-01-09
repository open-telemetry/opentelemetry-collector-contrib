// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestConditionalAttributeSetApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		cond   ConditionalAttributeSet
		check  string
		attr   pcommon.Map
		expect pcommon.Map
	}{
		{
			name:  "No changes defined",
			cond:  NewConditionalAttributeSet[string](map[string]string{}),
			check: "database operation",
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Not matched check value",
			cond: NewConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				"application start",
			),
			check: "datatbase operation",
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "No condition set, applys to all",
			cond: NewConditionalAttributeSet[string](
				map[string]string{
					"service.version": "application.version",
				},
			),
			check: "datatbase operation",
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.version", "v0.0.0")
			}),
		},
		{
			name: "Matched condition, setting value",
			cond: NewConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				"application start",
				"application stop",
			),
			check: "application start",
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.version", "v0.0.0")
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, tc.cond.Do(StateSelectorApply, tc.attr, tc.check))
			assert.Equal(t, tc.expect.AsRaw(), tc.attr.AsRaw(), "Must match the expected value")
		})
	}
}

func TestConditionalAttributeSetRollback(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		cond   ConditionalAttributeSet
		check  string
		attr   pcommon.Map
		expect pcommon.Map
	}{
		{
			name:  "No changes defined",
			cond:  NewConditionalAttributeSet[string](map[string]string{}),
			check: "database operation",
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Not matched check value",
			cond: NewConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				"application start",
			),
			check: "datatbase operation",
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "No condition set, applys to all",
			cond: NewConditionalAttributeSet[string](
				map[string]string{
					"service.version": "application.version",
				},
			),
			check: "datatbase operation",
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Matched condition, setting value",
			cond: NewConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				"application start",
				"application stop",
			),
			check: "application start",
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, tc.cond.Do(StateSelectorRollback, tc.attr, tc.check))
			assert.Equal(t, tc.expect.AsRaw(), tc.attr.AsRaw(), "Must match the expected value")
		})
	}
}
