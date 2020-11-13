// Copyright The OpenTelemetry Authors
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

package serialization

import (
	"testing"

	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestSerializeIntDataPoints(t *testing.T) {
	type args struct {
		name string
		data pdata.IntDataPointSlice
		tags []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeIntDataPoints(tt.args.name, tt.args.data, tt.args.tags); got != tt.want {
				t.Errorf("SerializeIntDataPoints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeDoubleDataPoints(t *testing.T) {
	type args struct {
		name string
		data pdata.DoubleDataPointSlice
		tags []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeDoubleDataPoints(tt.args.name, tt.args.data, tt.args.tags); got != tt.want {
				t.Errorf("SerializeDoubleDataPoints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeDoubleHistogramMetrics(t *testing.T) {
	type args struct {
		name string
		data pdata.DoubleHistogramDataPointSlice
		tags []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeDoubleHistogramMetrics(tt.args.name, tt.args.data, tt.args.tags); got != tt.want {
				t.Errorf("SerializeDoubleHistogramMetrics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeIntHistogramMetrics(t *testing.T) {
	type args struct {
		name string
		data pdata.IntHistogramDataPointSlice
		tags []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeIntHistogramMetrics(tt.args.name, tt.args.data, tt.args.tags); got != tt.want {
				t.Errorf("SerializeIntHistogramMetrics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_serializeLine(t *testing.T) {
	type args struct {
		name      string
		tagline   string
		valueline string
		timestamp pdata.TimestampUnixNano
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Constructs a Dynatrace metrics ingest string",
			args: args{name: "metric_name", tagline: "tag=value", valueline: "gauge,60", timestamp: pdata.TimestampUnixNano(uint64(1e9))},
			want: "metric_name,tag=value gauge,60 1000000\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serializeLine(tt.args.name, tt.args.tagline, tt.args.valueline, tt.args.timestamp); got != tt.want {
				t.Errorf("serializeLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_serializeTags(t *testing.T) {
	type args struct {
		labels       pdata.StringMap
		exporterTags []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "No labels or tags",
			args: args{labels: pdata.NewStringMap()},
			want: "",
		},
		{
			name: "Labels with no tags",
			args: args{labels: pdata.NewStringMap().InitFromMap(map[string]string{"test": "value"})},
			want: "test=value",
		},
		{
			name: "Tags with no labels",
			args: args{labels: pdata.NewStringMap(), exporterTags: []string{"tag=value"}},
			want: "tag=value",
		},
		{
			name: "Tags and labels",
			args: args{labels: pdata.NewStringMap().InitFromMap(map[string]string{"test": "value"}), exporterTags: []string{"tag=value"}},
			want: "tag=value,test=value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serializeTags(tt.args.labels, tt.args.exporterTags); got != tt.want {
				t.Errorf("serializeTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeString(t *testing.T) {
	type args struct {
		str string
		max int
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "Valid strings are unchanged",
			args:    args{str: "valid", max: 5},
			want:    "valid",
			wantErr: false,
		},
		{
			name:    "Long strings are trimmed",
			args:    args{str: "toolong", max: 5},
			want:    "toolo",
			wantErr: false,
		},
		{
			name:    "Invalid characters are replaced",
			args:    args{str: "stringwith.!@#$%invalidchars", max: 50},
			want:    "stringwith._invalidchars",
			wantErr: false,
		},
		{
			name:    "Leading numbers are trimmed",
			args:    args{str: "0123startswithnumbers", max: 50},
			want:    "startswithnumbers",
			wantErr: false,
		},
		{
			name:    "Empty strings cause an error",
			args:    args{str: "", max: 5},
			want:    "",
			wantErr: true,
		},
		{
			name:    "Strings trimmed to nothing cause an error",
			args:    args{str: "_0.231", max: 5},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeString(tt.args.str, tt.args.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_serializeFloat64(t *testing.T) {
	type args struct {
		n float64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Serialize 0.0 to 0",
			args: args{n: 0.0},
			want: "0",
		},
		{
			name: "Serialize 1.0 to 1",
			args: args{n: 1.0},
			want: "1",
		},
		{
			name: "Serialize 1.1 to 1.1",
			args: args{n: 1.1},
			want: "1.1",
		},
		{
			name: "Serialize very small decimals",
			args: args{n: 1.0000000000000001},
			want: "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serializeFloat64(tt.args.n); got != tt.want {
				t.Errorf("serializeFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}
