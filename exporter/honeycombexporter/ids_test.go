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
	"encoding/hex"
	"reflect"
	"testing"
)

func TestGetHoneycombTraceID(t *testing.T) {
	tests := []struct {
		name    string
		traceID string
		want    string
	}{
		{
			name:    "64-bit traceID",
			traceID: "cbe4decd12429177",
			want:    "cbe4decd12429177",
		},
		{
			name:    "128-bit zero-padded traceID",
			traceID: "0000000000000000cbe4decd12429177",
			want:    "cbe4decd12429177",
		},
		{
			name:    "128-bit non-zero-padded traceID",
			traceID: "f23b42eac289a0fdcde48fcbe3ab1a32",
			want:    "f23b42eac289a0fdcde48fcbe3ab1a32",
		},
		{
			name:    "Non-hex traceID",
			traceID: "foobar1",
			want:    "666f6f62617231",
		},
		{
			name:    "Longer non-hex traceID",
			traceID: "foobarbaz",
			want:    "666f6f6261726261",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceID, err := hex.DecodeString(tt.traceID)
			if err != nil {
				traceID = []byte(tt.traceID)
			}
			got := getHoneycombTraceID(traceID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHoneycombTraceID:\n\tgot:  %#v\n\twant: %#v", got, tt.want)
			}
		})
	}
}
