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

	"go.opentelemetry.io/collector/model/pdata"
)

func TestSerializeIntDataPoints(t *testing.T) {
	type args struct {
		name string
		data pdata.NumberDataPointSlice
		tags []string
	}

	intSlice := pdata.NewNumberDataPointSlice()
	intPoint := intSlice.AppendEmpty()
	intPoint.SetIntVal(13)
	intPoint.SetTimestamp(pdata.Timestamp(100_000_000))
	intPoint1 := intSlice.AppendEmpty()
	intPoint1.SetIntVal(14)
	intPoint1.SetTimestamp(pdata.Timestamp(101_000_000))

	labelIntSlice := pdata.NewNumberDataPointSlice()
	labelIntPoint := labelIntSlice.AppendEmpty()
	labelIntPoint.SetIntVal(13)
	labelIntPoint.SetTimestamp(pdata.Timestamp(100_000_000))
	labelIntPoint.Attributes().InsertString("labelKey", "labelValue")

	emptyLabelIntSlice := pdata.NewNumberDataPointSlice()
	emptyLabelIntPoint := emptyLabelIntSlice.AppendEmpty()
	emptyLabelIntPoint.SetIntVal(13)
	emptyLabelIntPoint.SetTimestamp(pdata.Timestamp(100_000_000))
	emptyLabelIntPoint.Attributes().InsertString("emptyLabelKey", "")

	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Serialize integer data points",
			args: args{
				name: "my_int_gauge",
				data: intSlice,
				tags: []string{},
			},
			want: []string{"my_int_gauge 13 100", "my_int_gauge 14 101"},
		},
		{
			name: "Serialize integer data points with tags",
			args: args{
				name: "my_int_gauge_with_tags",
				data: intSlice,
				tags: []string{"test_key=testval"},
			},
			want: []string{"my_int_gauge_with_tags,test_key=testval 13 100", "my_int_gauge_with_tags,test_key=testval 14 101"},
		},
		{
			name: "Serialize integer data points with labels",
			args: args{
				name: "my_int_gauge_with_labels",
				data: labelIntSlice,
				tags: []string{},
			},
			want: []string{"my_int_gauge_with_labels,labelkey=\"labelValue\" 13 100"},
		},
		{
			name: "Serialize integer data points with empty label",
			args: args{
				name: "my_int_gauge_with_empty_labels",
				data: emptyLabelIntSlice,
				tags: []string{},
			},
			want: []string{"my_int_gauge_with_empty_labels,emptylabelkey=\"\" 13 100"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeNumberDataPoints(tt.args.name, tt.args.data, tt.args.tags); !equal(got, tt.want) {
				t.Errorf("SerializeNumberDataPoints() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSerializeDoubleDataPoints(t *testing.T) {
	doubleSlice := pdata.NewNumberDataPointSlice()
	doublePoint := doubleSlice.AppendEmpty()
	doublePoint.SetDoubleVal(13.1)
	doublePoint.SetTimestamp(pdata.Timestamp(100_000_000))

	labelDoubleSlice := pdata.NewNumberDataPointSlice()
	labelDoublePoint := labelDoubleSlice.AppendEmpty()
	labelDoublePoint.SetDoubleVal(13.1)
	labelDoublePoint.SetTimestamp(pdata.Timestamp(100_000_000))
	labelDoublePoint.Attributes().InsertString("labelKey", "labelValue")

	type args struct {
		name string
		data pdata.NumberDataPointSlice
		tags []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Serialize double data points",
			args: args{
				name: "my_double_gauge",
				data: doubleSlice,
				tags: []string{},
			},
			want: []string{"my_double_gauge 13.1 100"},
		},
		{
			name: "Serialize double data points with tags",
			args: args{
				name: "my_double_gauge_with_tags",
				data: doubleSlice,
				tags: []string{"test_key=testval"},
			},
			want: []string{"my_double_gauge_with_tags,test_key=testval 13.1 100"},
		},
		{
			name: "Serialize double data points with labels",
			args: args{
				name: "my_double_gauge_with_labels",
				data: labelDoubleSlice,
				tags: []string{},
			},
			want: []string{"my_double_gauge_with_labels,labelkey=\"labelValue\" 13.1 100"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeNumberDataPoints(tt.args.name, tt.args.data, tt.args.tags); !equal(got, tt.want) {
				t.Errorf("SerializeNumberDataPoints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeHistogramMetrics(t *testing.T) {
	doubleHistSlice := pdata.NewHistogramDataPointSlice()
	doubleHistPoint := doubleHistSlice.AppendEmpty()
	doubleHistPoint.SetCount(10)
	doubleHistPoint.SetSum(101.0)
	doubleHistPoint.SetTimestamp(pdata.Timestamp(100_000_000))

	labelDoubleHistSlice := pdata.NewHistogramDataPointSlice()
	labelDoubleHistPoint := labelDoubleHistSlice.AppendEmpty()
	labelDoubleHistPoint.SetCount(10)
	labelDoubleHistPoint.SetSum(101.0)
	labelDoubleHistPoint.SetTimestamp(pdata.Timestamp(100_000_000))
	labelDoubleHistPoint.Attributes().InsertString("labelKey", "labelValue")

	zeroHistogramSlice := pdata.NewHistogramDataPointSlice()
	zeroHistogramDataPoint := zeroHistogramSlice.AppendEmpty()
	zeroHistogramDataPoint.SetCount(0)
	zeroHistogramDataPoint.SetSum(0)
	zeroHistogramDataPoint.SetTimestamp(pdata.Timestamp(100_000_000))

	type args struct {
		name string
		data pdata.HistogramDataPointSlice
		tags []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Serialize double histogram data points",
			args: args{
				name: "my_double_hist",
				data: doubleHistSlice,
				tags: []string{},
			},
			want: []string{"my_double_hist gauge,min=10.1,max=10.1,sum=101,count=10 100"},
		},
		{
			name: "Serialize double histogram data points with tags",
			args: args{
				name: "my_double_hist_with_tags",
				data: doubleHistSlice,
				tags: []string{"test_key=testval"},
			},
			want: []string{"my_double_hist_with_tags,test_key=testval gauge,min=10.1,max=10.1,sum=101,count=10 100"},
		},
		{
			name: "Serialize double histogram data points with labels",
			args: args{
				name: "my_double_hist_with_labels",
				data: labelDoubleHistSlice,
				tags: []string{},
			},
			want: []string{"my_double_hist_with_labels,labelkey=\"labelValue\" gauge,min=10.1,max=10.1,sum=101,count=10 100"},
		},
		{
			name: "Serialize zero double histogram",
			args: args{
				name: "zero_double_hist",
				data: zeroHistogramSlice,
				tags: []string{},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeHistogramMetrics(tt.args.name, tt.args.data, tt.args.tags); !equal(got, tt.want) {
				t.Errorf("SerializeHistogramMetrics() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_serializeLine(t *testing.T) {
	type args struct {
		name      string
		tagline   string
		valueline string
		timestamp pdata.Timestamp
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Constructs a Dynatrace metrics ingest string",
			args: args{name: "metric_name", tagline: "tag=value", valueline: "gauge,60", timestamp: pdata.Timestamp(uint64(100_000_000))},
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
		attributes   pdata.AttributeMap
		exporterTags []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "No attributes or tags",
			args: args{attributes: pdata.NewAttributeMap()},
			want: "",
		},
		{
			name: "Attributes with no tags",
			args: args{attributes: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{"test": pdata.NewAttributeValueString("value")})},
			want: "test=\"value\"",
		},
		{
			name: "Tags with no attributes",
			args: args{attributes: pdata.NewAttributeMap(), exporterTags: []string{"tag=value"}},
			want: "tag=value",
		},
		{
			name: "Tags and attributes",
			args: args{attributes: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{"test": pdata.NewAttributeValueString("value")}), exporterTags: []string{"tag=value"}},
			want: "tag=value,test=\"value\"",
		},
		{
			name: "Invalid tags",
			args: args{attributes: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{"_": pdata.NewAttributeValueString("value")}), exporterTags: []string{"tag=value"}},
			want: "tag=value",
		},
		{
			name: "Tag with trailing _",
			args: args{attributes: pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{"test__": pdata.NewAttributeValueString("value")}), exporterTags: []string{"tag=value"}},
			want: "tag=value,test=\"value\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serializeTags(tt.args.attributes, tt.args.exporterTags); got != tt.want {
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

func equal(a, b []string) bool {
	if len(a) == len(b) {
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	return false
}
