// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate

import (
"testing"

"github.com/stretchr/testify/assert"
"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

func TestConditionalLambdaAttributeSetRollback(t *testing.T) {
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
				m.PutStr("application.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("tester", "yes")
				m.PutStr("service.version", "v0.0.0")
			}),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			logs := plog.NewResourceLogs()
			tc.in.CopyTo(logs.Resource().Attributes())
			cond := NewConditionalLambdaAttributeSet[plog.ResourceLogs](tc.schemaChange, tc.testFunc)

			assert.NoError(t, cond.Rollback(tc.in, logs))
			assert.Equal(t, tc.expect.AsRaw(), tc.in.AsRaw(), "Must match the expected value")
		})
	}
}

func TestConditionalLambdaAttributeSetSliceApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		slice  *ConditionalLambdaAttributeSetSlice[pmetric.ResourceMetrics]
		in     pcommon.Map
		expect pcommon.Map
	}{
		{
			name:  "No changes",
			slice: NewConditionalLambdaAttributeSetSlice(
				NewConditionalLambdaAttributeSet(map[string]string{}, func(resource pmetric.ResourceMetrics) bool {
					return true
				}),
			),
			in: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Not matched check value",
			slice: NewConditionalLambdaAttributeSetSlice(
				NewConditionalLambdaAttributeSet(
					map[string]string{
						"service_version": "service.version",
					},
					func(resource pmetric.ResourceMetrics) bool {
						return true
					},
				),
				NewConditionalLambdaAttributeSet(
					map[string]string{
						"service.version": "shark.attack",
					},
					func(resource pmetric.ResourceMetrics) bool {
						return false
					},
				),
			),
			in: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			metrics := pmetric.NewResourceMetrics()
			tc.in.CopyTo(metrics.Resource().Attributes())

			assert.NoError(t, tc.slice.Apply(tc.in, metrics))
			assert.Equal(t, tc.expect.AsRaw(), tc.in.AsRaw(), "Must match the expected values")
		})
	}
}

func TestConditionalLambdaAttributeSetSliceRollback(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		slice  *ConditionalLambdaAttributeSetSlice[pmetric.ResourceMetrics]
		in     pcommon.Map
		expect pcommon.Map
	}{
		{
			name:  "No changes",
			slice: NewConditionalLambdaAttributeSetSlice(
				NewConditionalLambdaAttributeSet(map[string]string{}, func(resource pmetric.ResourceMetrics) bool {
					return true
				}),
			),
			in: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
		},
		{
			name: "Not matched check value",
			slice: NewConditionalLambdaAttributeSetSlice(
				NewConditionalLambdaAttributeSet(
					map[string]string{
						"service_version": "service.version",
					},
					func(resource pmetric.ResourceMetrics) bool {
						return true
					},
				),
				NewConditionalLambdaAttributeSet(
					map[string]string{
						"service.version": "shark.attack",
					},
					func(resource pmetric.ResourceMetrics) bool {
						return false
					},
				),
			),
			in: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.0")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.0")
			}),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			metrics := pmetric.NewResourceMetrics()
			tc.in.CopyTo(metrics.Resource().Attributes())

			assert.NoError(t, tc.slice.Rollback(tc.in, metrics))
			assert.Equal(t, tc.expect.AsRaw(), tc.in.AsRaw(), "Must match the expected values")
		})
	}
}
