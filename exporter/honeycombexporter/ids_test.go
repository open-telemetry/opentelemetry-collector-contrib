// Copyright 2019 OpenTelemetry Authors
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

package honeycombexporter

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/collector/model/pdata"
)

func TestGetHoneycombTraceID(t *testing.T) {
	tests := []struct {
		name    string
		traceID pdata.TraceID
		want    string
	}{
		{
			name:    "128-bit zero-padded traceID",
			traceID: pdata.NewTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 203, 228, 222, 205, 18, 66, 145, 119}),
			want:    "cbe4decd12429177",
		},
		{
			name:    "128-bit non-zero-padded traceID",
			traceID: pdata.NewTraceID([16]byte{242, 59, 66, 234, 194, 137, 160, 253, 205, 228, 143, 203, 227, 171, 26, 50}),
			want:    "f23b42eac289a0fdcde48fcbe3ab1a32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getHoneycombTraceID(tt.traceID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHoneycombTraceID:\n\tgot:  %#v\n\twant: %#v", got, tt.want)
			}
		})
	}
}
