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

package tqlcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

func TestScopePathGetSetter(t *testing.T) {
	refIS := createInstrumentationScope()

	tests := []struct {
		name     string
		path     []tql.Field
		orig     interface{}
		newVal   interface{}
		modified func(is pcommon.InstrumentationScope)
	}{
		{
			name:   "instrumentation_scope",
			path:   []tql.Field{},
			orig:   refIS,
			newVal: pcommon.NewInstrumentationScope(),
			modified: func(is pcommon.InstrumentationScope) {
				pcommon.NewInstrumentationScope().CopyTo(is)
			},
		},
		{
			name: "instrumentation_scope name",
			path: []tql.Field{
				{
					Name: "name",
				},
			},
			orig:   refIS.Name(),
			newVal: "park",
			modified: func(is pcommon.InstrumentationScope) {
				is.SetName("park")
			},
		},
		{
			name: "instrumentation_scope version",
			path: []tql.Field{
				{
					Name: "version",
				},
			},
			orig:   refIS.Version(),
			newVal: "next",
			modified: func(is pcommon.InstrumentationScope) {
				is.SetVersion("next")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := ScopePathGetSetter(tt.path)
			assert.NoError(t, err)

			is := createInstrumentationScope()

			got := accessor.Get(newInstrumentationScopeContext(is))
			assert.Equal(t, tt.orig, got)

			accessor.Set(newInstrumentationScopeContext(is), tt.newVal)

			expectedIS := createInstrumentationScope()
			tt.modified(expectedIS)

			assert.Equal(t, expectedIS, is)
		})
	}
}

func createInstrumentationScope() pcommon.InstrumentationScope {
	is := pcommon.NewInstrumentationScope()
	is.SetName("library")
	is.SetVersion("version")
	return is
}

type instrumentationScopeContext struct {
	is pcommon.InstrumentationScope
}

func (r *instrumentationScopeContext) GetItem() interface{} {
	return nil
}

func (r *instrumentationScopeContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return r.is
}

func (r *instrumentationScopeContext) GetResource() pcommon.Resource {
	return pcommon.Resource{}
}

func newInstrumentationScopeContext(is pcommon.InstrumentationScope) tql.TransformContext {
	return &instrumentationScopeContext{is: is}
}
