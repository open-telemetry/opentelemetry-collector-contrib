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
	"errors"
	"testing"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
)

func Test_StatsDParser_Parse(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantMetricName  string
		wantMetricValue interface{}
		err             error
	}{
		{
			name:  "empty input string",
			input: "",
			err:   errors.New("invalid message format: "),
		},
		{
			name:  "missing metric value",
			input: "test.metric|c",
			err:   errors.New("invalid <name>:<value> format: test.metric"),
		},
		{
			name:  "empty metric name",
			input: ":42|c",
			err:   errors.New("empty metric name"),
		},
		{
			name:  "empty metric value",
			input: "test.metric:|c",
			err:   errors.New("empty metric value"),
		},
		{
			name:            "integer counter",
			input:           "test.metric:42|c",
			wantMetricName:  "test.metric",
			wantMetricValue: &metricspb.Point_Int64Value{Int64Value: 42},
		},
		{
			name:            "gracefully handle float counter value",
			input:           "test.metric:42.0|c",
			wantMetricName:  "test.metric",
			wantMetricValue: &metricspb.Point_Int64Value{Int64Value: 42},
		},
		{
			name:  "invalid metric value",
			input: "test.metric:42.abc|c",
			err:   errors.New("parse metric value string: 42.abc"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &StatsDParser{}

			got, err := p.Parse(tt.input)

			if tt.err != nil {
				assert.Equal(t, err, tt.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, got.GetTimeseries()[0].GetPoints()[0].GetValue(), tt.wantMetricValue)
				assert.Equal(t, got.GetMetricDescriptor().GetName(), tt.wantMetricName)
			}
		})
	}
}
