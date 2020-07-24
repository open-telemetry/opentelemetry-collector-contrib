// Copyright 2020, OpenTelemetry Authors
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

package protocol

import (
	"testing"

	// TODO: don't use the opencensus-proto package???
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
)

func Test_StatsDParser_Parse(t *testing.T) {
	// TODO: fill in StatsD message parsing test cases
	tests := []struct {
		name  string
		input string
		want  *metricspb.Metric
		err   error
	}{
		{
			name: "test",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &StatsDParser{}

			got, err := p.Parse(tt.input)

			if tt.err != nil {
				assert.Equal(t, err, tt.err)
			} else {
				assert.Equal(t, got, tt.want)
			}
		})
	}
}
