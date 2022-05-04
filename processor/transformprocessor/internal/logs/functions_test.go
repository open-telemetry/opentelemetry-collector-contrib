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

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_newFunctionCall(t *testing.T) {
	input := plog.NewLogRecord()
	attrs := pcommon.NewMap()
	attrs.InsertString("test", "1")
	attrs.InsertInt("test2", 3)
	attrs.InsertBool("test3", true)
	attrs.CopyTo(input.Attributes())

	tests := []struct {
		name string
		inv  common.Invocation
		want func(plog.LogRecord)
	}{
		{
			name: "set name",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "severity_text",
								},
							},
						},
					},
					{
						String: strp("fail"),
					},
				},
			},
			want: func(log plog.LogRecord) {
				input.CopyTo(log)
				log.SetSeverityText("fail")
			},
		},
		{
			name: "keep_keys one",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
				},
			},
			want: func(log plog.LogRecord) {
				input.CopyTo(log)
				log.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "1")
				attrs.CopyTo(log.Attributes())
			},
		},
		{
			name: "keep_keys two",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
					{
						String: strp("test2"),
					},
				},
			},
			want: func(log plog.LogRecord) {
				input.CopyTo(log)
				log.Attributes().Clear()
				attrs := pcommon.NewMap()
				attrs.InsertString("test", "1")
				attrs.InsertInt("test2", 3)
				attrs.CopyTo(log.Attributes())
			},
		},
		{
			name: "keep_keys none",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
				},
			},
			want: func(log plog.LogRecord) {
				input.CopyTo(log)
				log.Attributes().Clear()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := plog.NewLogRecord()
			input.CopyTo(log)

			evaluate, err := common.NewFunctionCall(tt.inv, DefaultFunctions(), ParsePath)
			assert.NoError(t, err)
			evaluate(logTransformContext{
				log:      log,
				il:       pcommon.NewInstrumentationScope(),
				resource: pcommon.NewResource(),
			})

			expected := plog.NewLogRecord()
			tt.want(expected)
			assert.Equal(t, expected, log)
		})
	}
}
