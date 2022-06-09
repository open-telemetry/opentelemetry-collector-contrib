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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
)

func Test_newConditionEvaluator(t *testing.T) {
	tests := []struct {
		name string
		cond *Condition
		item interface{}
	}{
		{
			name: "literals match",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello"),
				},
				Right: Value{
					String: testhelper.Strp("hello"),
				},
				Op: "==",
			},
		},
		{
			name: "literals don't match",
			cond: &Condition{
				Left: Value{
					String: testhelper.Strp("hello"),
				},
				Right: Value{
					String: testhelper.Strp("goodbye"),
				},
				Op: "!=",
			},
		},
		{
			name: "path expression matches",
			cond: &Condition{
				Left: Value{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
				Right: Value{
					String: testhelper.Strp("bear"),
				},
				Op: "==",
			},
			item: "bear",
		},
		{
			name: "path expression not matches",
			cond: &Condition{
				Left: Value{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
				Right: Value{
					String: testhelper.Strp("cat"),
				},
				Op: "!=",
			},
			item: "bear",
		},
		{
			name: "no condition",
			cond: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := newConditionEvaluator(tt.cond, DefaultFunctions(), testParsePath)
			assert.NoError(t, err)
			assert.True(t, evaluate(testhelper.TestTransformContext{
				Item: tt.item,
			}))
		})
	}

	t.Run("invalid", func(t *testing.T) {
		_, err := newConditionEvaluator(&Condition{
			Left: Value{
				String: testhelper.Strp("bear"),
			},
			Op: "<>",
			Right: Value{
				String: testhelper.Strp("cat"),
			},
		}, DefaultFunctions(), testParsePath)
		assert.Error(t, err)
	})
}
