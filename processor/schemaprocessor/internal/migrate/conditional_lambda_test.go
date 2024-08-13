// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate

import (
"testing"

"github.com/stretchr/testify/assert"
"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
)

func TestConditionalLambdaAttributeSetApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		testFunc  ResourceTestFunc[plog.ResourceLogs]
		schemaChange ast.AttributeMap
		in           pcommon.Map
		expect       pcommon.Map
	}{
		{
			name:  "No changes defined",
			testFunc: func(resource plog.ResourceLogs) bool {
				return true
			},
			schemaChange: map[string]string{},
			in: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Lambda fails",
			testFunc: func(resource plog.ResourceLogs) bool {
				return false
			},
			schemaChange: map[string]string{
				"service.version": "application.version",
			},
			in: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Matched condition, setting value",
			testFunc: func(resource plog.ResourceLogs) bool {
				_, ok := resource.Resource().Attributes().Get("tester")
				return ok
			},
			schemaChange: map[string]string{
					"service.version": "application.version",
			},
			in: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("tester", "yes")
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("tester", "yes")
				m.PutStr("application.version", "v0.0.0")
			}),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			logs := plog.NewResourceLogs()
			tc.in.CopyTo(logs.Resource().Attributes())
			cond := NewConditionalLambdaAttributeSet[plog.ResourceLogs](tc.schemaChange, tc.testFunc)

			assert.NoError(t, cond.Apply(tc.in, logs))
			assert.Equal(t, tc.expect.AsRaw(), tc.in.AsRaw(), "Must match the expected value")
		})
	}
}

//func TestConditionalLambdaAttributeSetRollback(t *testing.T) {
//	t.Parallel()
//
//	for _, tc := range []struct {
//		name   string
//		cond   *ConditionalLambdaAttributeSet
//		check  string
//		attr   pcommon.Map
//		expect pcommon.Map
//	}{
//		{
//			name:  "No changes defined",
//			cond:  NewConditionalLambdaAttributeSet[string](map[string]string{}),
//			check: "database operation",
//			attr: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//			expect: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//		},
//		{
//			name: "Not matched check value",
//			cond: NewConditionalLambdaAttributeSet(
//				map[string]string{
//					"service.version": "application.version",
//				},
//				"application start",
//			),
//			check: "datatbase operation",
//			attr: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//			expect: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//		},
//		{
//			name: "No condition set, applys to all",
//			cond: NewConditionalLambdaAttributeSet[string](
//				map[string]string{
//					"service.version": "application.version",
//				},
//			),
//			check: "datatbase operation",
//			attr: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("application.version", "v0.0.0")
//			}),
//			expect: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//		},
//		{
//			name: "Matched condition, setting value",
//			cond: NewConditionalLambdaAttributeSet(
//				map[string]string{
//					"service.version": "application.version",
//				},
//				"application start",
//				"application stop",
//			),
//			check: "application start",
//			attr: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("application.version", "v0.0.0")
//			}),
//			expect: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//		},
//	} {
//		tc := tc
//		t.Run(tc.name, func(t *testing.T) {
//			t.Parallel()
//
//			assert.NoError(t, tc.cond.Rollback(tc.attr, tc.check))
//			assert.Equal(t, tc.expect.AsRaw(), tc.attr.AsRaw(), "Must match the expected value")
//		})
//	}
//}
//
//func TestConditionalLambdaAttribueSetSliceApply(t *testing.T) {
//	t.Parallel()
//
//	for _, tc := range []struct {
//		name   string
//		slice  *ConditionalLambdaAttributeSetSlice
//		check  string
//		attrs  pcommon.Map
//		expect pcommon.Map
//	}{
//		{
//			name:  "No changes",
//			slice: NewConditionalLambdaAttributeSetSlice(),
//			check: "application start",
//			attrs: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//			expect: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//		},
//		{
//			name: "Not matched check value",
//			slice: NewConditionalLambdaAttributeSetSlice(
//				NewConditionalLambdaAttributeSet[string](
//					map[string]string{
//						"service_version": "service.version",
//					},
//				),
//				// intentially silly to be make it clear
//				// that this should not be applied
//				NewConditionalLambdaAttributeSet(
//					map[string]string{
//						"service.version": "shark.attack",
//					},
//					"shark spotted",
//				),
//			),
//			check: "application start",
//			attrs: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service_version", "v0.0.0")
//			}),
//			expect: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//		},
//	} {
//		tc := tc
//		t.Run(tc.name, func(t *testing.T) {
//			t.Parallel()
//
//			assert.NoError(t, tc.slice.Apply(tc.attrs, tc.check))
//			assert.Equal(t, tc.expect.AsRaw(), tc.attrs.AsRaw(), "Must match the expected values")
//		})
//	}
//}
//
//func TestConditionalLambdaAttribueSetSliceRollback(t *testing.T) {
//	t.Parallel()
//
//	for _, tc := range []struct {
//		name   string
//		slice  *ConditionalLambdaAttributeSetSlice
//		check  string
//		attrs  pcommon.Map
//		expect pcommon.Map
//	}{
//		{
//			name:  "No changes",
//			slice: NewConditionalLambdaAttributeSetSlice(),
//			check: "application start",
//			attrs: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//			expect: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//		},
//		{
//			name: "Not matched check value",
//			slice: NewConditionalLambdaAttributeSetSlice(
//				NewConditionalLambdaAttributeSet[string](
//					map[string]string{
//						"service_version": "service.version",
//					},
//				),
//				// intentially silly to be make it clear
//				// that this should not be applied
//				NewConditionalLambdaAttributeSet(
//					map[string]string{
//						"service.version": "shark.attack",
//					},
//					"shark spotted",
//				),
//			),
//			check: "application start",
//			attrs: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service.version", "v0.0.0")
//			}),
//			expect: testHelperBuildMap(func(m pcommon.Map) {
//				m.PutStr("service_version", "v0.0.0")
//			}),
//		},
//	} {
//		tc := tc
//		t.Run(tc.name, func(t *testing.T) {
//			t.Parallel()
//
//			assert.NoError(t, tc.slice.Rollback(tc.attrs, tc.check))
//			assert.Equal(t, tc.expect.AsRaw(), tc.attrs.AsRaw(), "Must match the expected values")
//		})
//	}
//}
