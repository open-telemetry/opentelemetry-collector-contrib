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

		acs := NewAttributeChangeSet(map[string]string{
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
}

func TestAttributeChangeSetApply(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		acs    *AttributeChangeSet
		attrs  pcommon.Map
		expect pcommon.Map
		errVal string
	}{
		{
			name: "no modifications",
			acs:  NewAttributeChangeSet(map[string]string{}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutInt("test.cases", 1)
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutInt("test.cases", 1)
			}),
		},
		{
			name: "Apply changes",
			acs: NewAttributeChangeSet(map[string]string{
				"service_version": "service.version",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
		},
		{
			name: "naming loop",
			acs: NewAttributeChangeSet(map[string]string{
				"service.version": "service_version",
				"service_version": "service.version",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
		},
		{
			name: "overrides existing value",
			acs: NewAttributeChangeSet(map[string]string{
				"application.name": "service.name",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.name", "my-awesome-application")
				m.PutStr("service.name", "my-awesome-service")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.name", "my-awesome-application")
			}),
			errVal: "value \"service.name\" already exists",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.acs.Apply(tc.attrs)
			if tc.errVal == "" {
				assert.NoError(t, err, "Must not return an error")
			} else {
				assert.EqualError(t, err, tc.errVal, "Must match the expected error string")
			}
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
		errVal string
	}{
		{
			name: "no modifications",
			acs:  NewAttributeChangeSet(map[string]string{}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutInt("test.cases", 1)
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutInt("test.cases", 1)
			}),
		},
		{
			name: "Apply changes",
			acs: NewAttributeChangeSet(map[string]string{
				"service_version": "service.version",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
		},
		{
			name: "naming loop",
			acs: NewAttributeChangeSet(map[string]string{
				"service.version": "service_version",
				"service_version": "service.version",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.version", "v0.0.1")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service_version", "v0.0.1")
			}),
		},
		{
			name: "overrides existing value",
			acs: NewAttributeChangeSet(map[string]string{
				"application.name": "service.name",
			}),
			attrs: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("service.name", "my-awesome-application")
				m.PutStr("application.name", "my-awesome-service")
			}),
			expect: testHelperBuildMap(func(m pcommon.Map) {
				m.PutStr("application.name", "my-awesome-application")
			}),
			errVal: "value \"application.name\" already exists",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.acs.Rollback(tc.attrs)
			if tc.errVal == "" {
				assert.NoError(t, err, "Must not return an error")
			} else {
				assert.EqualError(t, err, tc.errVal, "Must match the expected error string")
			}
			assert.EqualValues(t, tc.expect.AsRaw(), tc.attrs.AsRaw(), "Must match the expected values")
		})
	}
}

func TestNewAttributeChangeSetSliceApply(t *testing.T) {
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
				NewAttributeChangeSet(map[string]string{
					"service_version": "service.version",
				}),
				NewAttributeChangeSet(map[string]string{
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

			assert.NoError(t, tc.changes.Apply(tc.attr))
			assert.Equal(t, tc.expect.AsRaw(), tc.attr.AsRaw(), "Must match the expected attributes")
		})
	}
}

func TestNewAttributeChangeSetSliceApplyRollback(t *testing.T) {
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
				NewAttributeChangeSet(map[string]string{
					"service_version": "service.version",
				}),
				NewAttributeChangeSet(map[string]string{
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

			assert.NoError(t, tc.changes.Rollback(tc.attr))
			assert.Equal(t, tc.expect.AsRaw(), tc.attr.AsRaw(), "Must match the expected attributes")
		})
	}
}
