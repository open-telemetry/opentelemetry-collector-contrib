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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_traceID(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		want  pcommon.TraceID
	}{
		{
			name:  "create trace id",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			want:  pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx := ottltest.TestTransformContext{}

			exprFunc, _ := TraceID(tt.bytes)
			actual := exprFunc(ctx)

			assert.Equal(t, tt.want, actual)
		})
	}
}

func Test_traceID_validation(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
	}{
		{
			name:  "byte slice less than 16",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		},
		{
			name:  "byte slice longer than 16",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := TraceID(tt.bytes)
			assert.Error(t, err, "traces ids must be 16 bytes")
		})
	}
}
