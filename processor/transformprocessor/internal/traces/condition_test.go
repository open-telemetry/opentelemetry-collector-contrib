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

package traces

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_newConditionEvaluator(t *testing.T) {
	span := pdata.NewSpan()
	span.SetName("bear")
	tests := []struct {
		name     string
		cond     *common.Condition
		matching pdata.Span
	}{
		{
			name: "literals match",
			cond: &common.Condition{
				Left: common.Value{
					String: strp("hello"),
				},
				Right: common.Value{
					String: strp("hello"),
				},
				Op: "==",
			},
			matching: span,
		},
		{
			name: "literals don't match",
			cond: &common.Condition{
				Left: common.Value{
					String: strp("hello"),
				},
				Right: common.Value{
					String: strp("goodbye"),
				},
				Op: "!=",
			},
			matching: span,
		},
		{
			name: "path expression matches",
			cond: &common.Condition{
				Left: common.Value{
					Path: &common.Path{
						Fields: []common.Field{
							{
								Name: "name",
							},
						},
					},
				},
				Right: common.Value{
					String: strp("bear"),
				},
				Op: "==",
			},
			matching: span,
		},
		{
			name: "path expression not matches",
			cond: &common.Condition{
				Left: common.Value{
					Path: &common.Path{
						Fields: []common.Field{
							{
								Name: "name",
							},
						},
					},
				},
				Right: common.Value{
					String: strp("cat"),
				},
				Op: "!=",
			},
			matching: span,
		},
		{
			name:     "no condition",
			cond:     nil,
			matching: span,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluate, err := newConditionEvaluator(tt.cond, DefaultFunctions())
			assert.NoError(t, err)
			assert.True(t, evaluate(tt.matching, pdata.NewInstrumentationLibrary(), pdata.NewResource()))
		})
	}

	t.Run("invalid", func(t *testing.T) {
		_, err := newConditionEvaluator(&common.Condition{
			Left: common.Value{
				String: strp("bear"),
			},
			Op: "<>",
			Right: common.Value{
				String: strp("cat"),
			},
		}, DefaultFunctions())
		assert.Error(t, err)
	})
}

func strp(s string) *string {
	return &s
}

func intp(i int64) *int64 {
	return &i
}

func floatp(f float64) *float64 {
	return &f
}
