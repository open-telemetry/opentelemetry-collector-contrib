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

	intSlice := pdata.NewIntDataPointSlice()
	intSlice.Resize(1)
	intPoint := intSlice.At(0)
	intPoint.SetValue(13)
	intPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	labelIntSlice := pdata.NewIntDataPointSlice()
	labelIntSlice.Resize(1)
	labelIntPoint := labelIntSlice.At(0)
	labelIntPoint.SetValue(13)
	labelIntPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))
	labelIntPoint.LabelsMap().Insert("labelKey", "labelValue")

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Serialize integer data points",
			args: args{
				name: "my_int_gauge",
				data: intSlice,
				tags: []string{},
			},
			want: "my_int_gauge 13 100",
		},
		{
			name: "Serialize integer data points with tags",
			args: args{
				name: "my_int_gauge_with_tags",
				data: intSlice,
				tags: []string{"test_key=testval"},
			},
			want: "my_int_gauge_with_tags,test_key=testval 13 100",
		},
		{
			name: "Serialize integer data points with labels",
			args: args{
				name: "my_int_gauge_with_labels",
				data: labelIntSlice,
				tags: []string{},
			},
			want: "my_int_gauge_with_labels,labelkey=\"labelValue\" 13 100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeIntDataPoints(tt.args.name, tt.args.data, tt.args.tags); got != tt.want {
				t.Errorf("SerializeIntDataPoints() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSerializeDoubleDataPoints(t *testing.T) {
	doubleSlice := pdata.NewDoubleDataPointSlice()
	doubleSlice.Resize(1)
	doublePoint := doubleSlice.At(0)
	doublePoint.SetValue(13.1)
	doublePoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	labelDoubleSlice := pdata.NewDoubleDataPointSlice()
	labelDoubleSlice.Resize(1)
	labelDoublePoint := labelDoubleSlice.At(0)
	labelDoublePoint.SetValue(13.1)
	labelDoublePoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))
	labelDoublePoint.LabelsMap().Insert("labelKey", "labelValue")

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
		{
			name: "Serialize double data points",
			args: args{
				name: "my_double_gauge",
				data: doubleSlice,
				tags: []string{},
			},
			want: "my_double_gauge 13.1 100",
		},
		{
			name: "Serialize double data points with tags",
			args: args{
				name: "my_double_gauge_with_tags",
				data: doubleSlice,
				tags: []string{"test_key=testval"},
			},
			want: "my_double_gauge_with_tags,test_key=testval 13.1 100",
		},
		{
			name: "Serialize double data points with labels",
			args: args{
				name: "my_double_gauge_with_labels",
				data: labelDoubleSlice,
				tags: []string{},
			},
			want: "my_double_gauge_with_labels,labelkey=\"labelValue\" 13.1 100",
		},
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
	doubleHistSlice := pdata.NewDoubleHistogramDataPointSlice()
	doubleHistSlice.Resize(1)
	doubleHistPoint := doubleHistSlice.At(0)
	doubleHistPoint.SetCount(10)
	doubleHistPoint.SetSum(101.0)
	doubleHistPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	labelDoubleHistSlice := pdata.NewDoubleHistogramDataPointSlice()
	labelDoubleHistSlice.Resize(1)
	labelDoubleHistPoint := labelDoubleHistSlice.At(0)
	labelDoubleHistPoint.SetCount(10)
	labelDoubleHistPoint.SetSum(101.0)
	labelDoubleHistPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))
	labelDoubleHistPoint.LabelsMap().Insert("labelKey", "labelValue")

	zeroDoubleHistogramSlice := pdata.NewDoubleHistogramDataPointSlice()
	zeroDoubleHistogramSlice.Resize(1)
	zeroDoubleHistogramDataPoint := zeroDoubleHistogramSlice.At(0)
	zeroDoubleHistogramDataPoint.SetCount(0)
	zeroDoubleHistogramDataPoint.SetSum(0)
	zeroDoubleHistogramDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

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
		{
			name: "Serialize double histogram data points",
			args: args{
				name: "my_double_hist",
				data: doubleHistSlice,
				tags: []string{},
			},
			want: "my_double_hist gauge,min=10.1,max=10.1,sum=101,count=10 100",
		},
		{
			name: "Serialize double histogram data points with tags",
			args: args{
				name: "my_double_hist_with_tags",
				data: doubleHistSlice,
				tags: []string{"test_key=testval"},
			},
			want: "my_double_hist_with_tags,test_key=testval gauge,min=10.1,max=10.1,sum=101,count=10 100",
		},
		{
			name: "Serialize double histogram data points with labels",
			args: args{
				name: "my_double_hist_with_labels",
				data: labelDoubleHistSlice,
				tags: []string{},
			},
			want: "my_double_hist_with_labels,labelkey=\"labelValue\" gauge,min=10.1,max=10.1,sum=101,count=10 100",
		},
		{
			name: "Serialize zero double histogram",
			args: args{
				name: "zero_double_hist",
				data: zeroDoubleHistogramSlice,
				tags: []string{},
			},
			want: "",
		},
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
	intHistSlice := pdata.NewIntHistogramDataPointSlice()
	intHistSlice.Resize(1)
	intHistPoint := intHistSlice.At(0)
	intHistPoint.SetCount(10)
	intHistPoint.SetSum(110)
	intHistPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

	labelIntHistSlice := pdata.NewIntHistogramDataPointSlice()
	labelIntHistSlice.Resize(1)
	labelIntHistPoint := labelIntHistSlice.At(0)
	labelIntHistPoint.SetCount(10)
	labelIntHistPoint.SetSum(110)
	labelIntHistPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))
	labelIntHistPoint.LabelsMap().Insert("labelKey", "labelValue")

	zeroIntHistogramSlice := pdata.NewIntHistogramDataPointSlice()
	zeroIntHistogramSlice.Resize(1)
	zeroIntHistogramDataPoint := zeroIntHistogramSlice.At(0)
	zeroIntHistogramDataPoint.SetCount(0)
	zeroIntHistogramDataPoint.SetSum(0)
	zeroIntHistogramDataPoint.SetTimestamp(pdata.TimestampUnixNano(100_000_000))

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
		{
			name: "Serialize integer histogram data points",
			args: args{
				name: "my_int_hist",
				data: intHistSlice,
				tags: []string{},
			},
			want: "my_int_hist gauge,min=11,max=11,sum=110,count=10 100",
		},
		{
			name: "Serialize integer histogram data points with tags",
			args: args{
				name: "my_int_hist_with_tags",
				data: intHistSlice,
				tags: []string{"test_key=testval"},
			},
			want: "my_int_hist_with_tags,test_key=testval gauge,min=11,max=11,sum=110,count=10 100",
		},
		{
			name: "Serialize integer histogram data points with labels",
			args: args{
				name: "my_int_hist_with_labels",
				data: labelIntHistSlice,
				tags: []string{},
			},
			want: "my_int_hist_with_labels,labelkey=\"labelValue\" gauge,min=11,max=11,sum=110,count=10 100",
		},
		{
			name: "Serialize zero integer histogram",
			args: args{
				name: "zero_int_hist",
				data: zeroIntHistogramSlice,
				tags: []string{},
			},
			want: "",
		},
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
			args: args{name: "metric_name", tagline: "tag=value", valueline: "gauge,60", timestamp: pdata.TimestampUnixNano(uint64(100_000_000))},
			want: "metric_name,tag=value gauge,60 100",
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
			want: "test=\"value\"",
		},
		{
			name: "Tags with no labels",
			args: args{labels: pdata.NewStringMap(), exporterTags: []string{"tag=value"}},
			want: "tag=value",
		},
		{
			name: "Tags and labels",
			args: args{labels: pdata.NewStringMap().InitFromMap(map[string]string{"test": "value"}), exporterTags: []string{"tag=value"}},
			want: "tag=value,test=\"value\"",
		},
		{
			name: "Invalid tags",
			args: args{labels: pdata.NewStringMap().InitFromMap(map[string]string{"_": "value"}), exporterTags: []string{"tag=value"}},
			want: "tag=value",
		},
		{
			name: "Tag with trailing _",
			args: args{labels: pdata.NewStringMap().InitFromMap(map[string]string{"test__": "value"}), exporterTags: []string{"tag=value"}},
			want: "tag=value,test=\"value\"",
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
			args:    args{str: "0.231", max: 5},
			want:    "",
			wantErr: true,
		},
		{
			name:    "Trailing _ are stripped",
			args:    args{str: "strip_", max: 5},
			want:    "strip",
			wantErr: false,
		},
		{
			name:    "Leading _ are not stripped",
			args:    args{str: "_strip", max: 6},
			want:    "_strip",
			wantErr: false,
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
