// Copyright The OpenTelemetry Authors
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

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_MergeMaps(t *testing.T) {

	input := pcommon.NewMap()
	input.PutStr("attr1", "value1")

	targetGetter := &ottl.StandardGetSetter[pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name   string
		source ottl.Getter[pcommon.Map]
		want   func(pcommon.Map)
	}{
		{
			name: "merge no conflicting keys",
			source: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "merge conflicting key",
			source: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("attr1", "value3")
					m.PutStr("attr2", "value2")
					return m, nil
				},
			},
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value3")
				expectedValue.PutStr("attr2", "value2")
			},
		},
		{
			name: "non-map value leaves target unchanged",
			source: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, _ pcommon.Map) (interface{}, error) {
					return nil, nil
				},
			},
			want: func(expectedValue pcommon.Map) {
				expectedValue.PutStr("attr1", "value1")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := MergeMaps[pcommon.Map](targetGetter, tt.source)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), scenarioMap)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}
