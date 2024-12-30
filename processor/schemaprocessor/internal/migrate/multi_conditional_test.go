// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestMultiConditionalAttributeSetApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		cond       MultiConditionalAttributeSet
		inCondData map[string]string
		inAttr     pcommon.Map
		expect     pcommon.Map
	}{
		{
			name:       "No changes defined",
			cond:       NewMultiConditionalAttributeSet[string](map[string]string{}, map[string][]string{}),
			inCondData: map[string]string{"span.name": "database operation"},
			inAttr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Not matched in value",
			cond: NewMultiConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				map[string][]string{"span.name": {"application start"}},
			),
			inCondData: map[string]string{"span.name": "datatbase operation"},
			inAttr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "No condition set, applys to all",
			cond: NewMultiConditionalAttributeSet[string](
				map[string]string{
					"service.version": "application.version",
				},
				map[string][]string{},
			),
			inCondData: map[string]string{"span.name": "datatbase operation"},
			inAttr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.version", "v0.0.0")
			}),
		},
		{
			name: "Matched one condition, setting value",
			cond: NewMultiConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				map[string][]string{
					"span.name": {"application start", "application end"},
				},
			),
			inCondData: map[string]string{"span.name": "application start"},
			inAttr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.version", "v0.0.0")
			}),
		},
		{
			name: "Matched one condition, other value, setting value",
			cond: NewMultiConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				map[string][]string{
					"span.name": {"application start", "application end"},
				},
			),
			inCondData: map[string]string{"span.name": "application end"},
			inAttr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.version", "v0.0.0")
			}),
		},
		{
			name: "Matched one out of two conditions, don't set value",
			cond: NewMultiConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				map[string][]string{
					"trace.name": {"application start"},
					"span.name":  {"application end"},
				},
			),
			inCondData: map[string]string{"span.name": "application start", "trace.name": "application end"},
			inAttr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Matched both conditions, set value",
			cond: NewMultiConditionalAttributeSet(
				map[string]string{
					"service.version": "application.version",
				},
				map[string][]string{
					"span.name":  {"application start"},
					"trace.name": {"application end"},
				},
			),
			inCondData: map[string]string{"span.name": "application start", "trace.name": "application end"},
			inAttr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.version", "v0.0.0")
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, tc.cond.Do(StateSelectorApply, tc.inAttr, tc.inCondData))
			assert.Equal(t, tc.expect.AsRaw(), tc.inAttr.AsRaw(), "Must match the expected value")
		})
	}
}
