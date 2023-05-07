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
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type testTypeDefKey string
type testTypeDefValue string

func testHelperBuildMap(opts ...func(pcommon.Map)) pcommon.Map {
	m := pcommon.NewMap()
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func TestNewAttributeChangeSet(t *testing.T) {
	t.Parallel()

	t.Run("map of strings", func(t *testing.T) {
		t.Parallel()

		acs := NewAttributes(map[string]string{
			"hello": "world",
		})

		expect := &AttributeChangeSet{
			updates: map[string]string{
				"hello": "world",
			},
			rollback: map[string]string{
				"world": "hello",
			},
		}

		assert.Equal(t, expect, acs, "Must match the expected value")
	})

	t.Run("typedef string values", func(t *testing.T) {
		t.Parallel()

		acs := NewAttributes(map[testTypeDefKey]testTypeDefValue{
			"awesome": "awesome.service",
		})

		expect := &AttributeChangeSet{
			updates: map[string]string{
				"awesome": "awesome.service",
			},
			rollback: map[string]string{
				"awesome.service": "awesome",
			},
		}

		assert.Equal(t, expect, acs, "Must match the expected value")
	})
}

func TestAttributeChangeSetApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		acs    *AttributeChangeSet
		attrs  pcommon.Map
		expect pcommon.Map
	}{
		{
			name: "no modifications",
			acs:  NewAttributes(map[string]string{}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutInt("test.cases", 1)
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutInt("test.cases", 1)
			}),
		},
		{
			name: "Apply changes",
			acs: NewAttributes(map[string]string{
				"service_version": "service.version",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
		},
		// Due to the way `pcommon.Map.RemoveIf` works,
		// it forces both values to be removed from the the attributes.
		{
			name: "naming loop",
			acs: NewAttributes(map[string]string{
				"service.version": "service_version",
				"service_version": "service.version",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
			expect: pcommon.NewMap(),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.acs.Apply(tc.attrs)
			assert.EqualValues(t, tc.expect.AsRaw(), tc.attrs.AsRaw(), "Must match the expected values")
		})
	}
}

func TestAttributeChangeSetRollback(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		acs    *AttributeChangeSet
		attrs  pcommon.Map
		expect pcommon.Map
	}{
		{
			name: "no modifications",
			acs:  NewAttributes(map[string]string{}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutInt("test.cases", 1)
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutInt("test.cases", 1)
			}),
		},
		{
			name: "Apply changes",
			acs: NewAttributes(map[string]string{
				"service_version": "service.version",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
		},
		// Due to the way `pcommon.Map.RemoveIf` works,
		// it forces both values to be removed from the the attributes.
		{
			name: "naming loop",
			acs: NewAttributes(map[string]string{
				"service.version": "service_version",
				"service_version": "service.version",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
			expect: pcommon.NewMap(),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.acs.Rollback(tc.attrs)
			assert.EqualValues(t, tc.expect.AsRaw(), tc.attrs.AsRaw(), "Must match the expected values")
		})
	}
}

func TestOrderedAttributeChangeSetsApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		changes *AttributeChangeSetSlice
		attr    pcommon.Map
		expect  pcommon.Map
	}{
		{
			name:    "no changes listed",
			changes: NewAttributeChangeSetSlice(),
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
		},
		{
			name: "changes defined",
			changes: NewAttributeChangeSetSlice(
				NewAttributes(map[string]string{
					"service_version": "service.version",
				}),
				NewAttributes(map[string]string{
					"service.version": "application.service.version",
				}),
			),
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.service.version", "v0.0.1")
			}),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.changes.Apply(tc.attr)
			assert.Equal(t, tc.expect.AsRaw(), tc.attr.AsRaw(), "Must match the expected attributes")
		})
	}
}

func TestOrderedAttributeChangeSetsRollback(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		changes *AttributeChangeSetSlice
		attr    pcommon.Map
		expect  pcommon.Map
	}{
		{
			name:    "no changes listed",
			changes: NewAttributeChangeSetSlice(),
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
		},
		{
			name: "changes defined",
			changes: NewAttributeChangeSetSlice(
				NewAttributes(map[string]string{
					"service_version": "service.version",
				}),
				NewAttributes(map[string]string{
					"service.version": "application.service.version",
				}),
			),
			attr: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.service.version", "v0.0.1")

			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.changes.Rollback(tc.attr)
			assert.Equal(t, tc.expect.AsRaw(), tc.attr.AsRaw(), "Must match the expected attributes")
		})
	}
}
