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
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/model/pdata"
)

var testTimestamp = pdata.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano())

func TestSerializeIntDataPoints(t *testing.T) {
	type args struct {
		name string
		data pdata.NumberDataPointSlice
		tags dimensions.NormalizedDimensionList
	}

	intSlice := pdata.NewNumberDataPointSlice()
	intPoint := intSlice.AppendEmpty()
	intPoint.SetIntVal(13)
	intPoint.SetTimestamp(testTimestamp)
	intPoint1 := intSlice.AppendEmpty()
	intPoint1.SetIntVal(14)
	// intPoint1.SetTimestamp(pdata.Timestamp(101_000_000))
	intPoint1.SetTimestamp(testTimestamp)

	labelIntSlice := pdata.NewNumberDataPointSlice()
	labelIntPoint := labelIntSlice.AppendEmpty()
	labelIntPoint.SetIntVal(13)
	labelIntPoint.SetTimestamp(testTimestamp)
	labelIntPoint.Attributes().InsertString("labelKey", "labelValue")

	emptyLabelIntSlice := pdata.NewNumberDataPointSlice()
	emptyLabelIntPoint := emptyLabelIntSlice.AppendEmpty()
	emptyLabelIntPoint.SetIntVal(13)
	emptyLabelIntPoint.SetTimestamp(testTimestamp)
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
				tags: dimensions.NewNormalizedDimensionList(),
			},
			want: []string{"my_int_gauge gauge,13 1626438600000", "my_int_gauge gauge,14 1626438600000"},
		},
		{
			name: "Serialize integer data points with tags",
			args: args{
				name: "my_int_gauge_with_tags",
				data: intSlice,
				tags: dimensions.NewNormalizedDimensionList(dimensions.NewDimension("test_key", "testval")),
			},
			want: []string{"my_int_gauge_with_tags,test_key=testval gauge,13 1626438600000", "my_int_gauge_with_tags,test_key=testval gauge,14 1626438600000"},
		},
		{
			name: "Serialize integer data points with labels",
			args: args{
				name: "my_int_gauge_with_labels",
				data: labelIntSlice,
				tags: dimensions.NewNormalizedDimensionList(),
			},
			want: []string{"my_int_gauge_with_labels,labelkey=labelValue gauge,13 1626438600000"},
		},
		{
			name: "Serialize integer data points with empty label",
			args: args{
				name: "my_int_gauge_with_empty_labels",
				data: emptyLabelIntSlice,
				tags: dimensions.NewNormalizedDimensionList(),
			},
			want: []string{"my_int_gauge_with_empty_labels,emptylabelkey= gauge,13 1626438600000"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeNumberDataPoints("", tt.args.name, tt.args.data, tt.args.tags); !equal(got, tt.want) {
				t.Errorf("SerializeNumberDataPoints() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSerializeDoubleDataPoints(t *testing.T) {
	doubleSlice := pdata.NewNumberDataPointSlice()
	doublePoint := doubleSlice.AppendEmpty()
	doublePoint.SetDoubleVal(13.1)
	doublePoint.SetTimestamp(testTimestamp)

	labelDoubleSlice := pdata.NewNumberDataPointSlice()
	labelDoublePoint := labelDoubleSlice.AppendEmpty()
	labelDoublePoint.SetDoubleVal(13.1)
	labelDoublePoint.SetTimestamp(testTimestamp)
	labelDoublePoint.Attributes().InsertString("labelKey", "labelValue")

	type args struct {
		name string
		data pdata.NumberDataPointSlice
		tags dimensions.NormalizedDimensionList
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
				tags: dimensions.NewNormalizedDimensionList(),
			},
			want: []string{"my_double_gauge gauge,13.1 1626438600000"},
		},
		{
			name: "Serialize double data points with tags",
			args: args{
				name: "my_double_gauge_with_tags",
				data: doubleSlice,
				tags: dimensions.NewNormalizedDimensionList(dimensions.NewDimension("test_key", "testval")),
			},
			want: []string{"my_double_gauge_with_tags,test_key=testval gauge,13.1 1626438600000"},
		},
		{
			name: "Serialize double data points with labels",
			args: args{
				name: "my_double_gauge_with_labels",
				data: labelDoubleSlice,
				tags: dimensions.NewNormalizedDimensionList(),
			},
			want: []string{"my_double_gauge_with_labels,labelkey=labelValue gauge,13.1 1626438600000"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeNumberDataPoints("", tt.args.name, tt.args.data, tt.args.tags); !equal(got, tt.want) {
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
	doubleHistPoint.SetTimestamp(testTimestamp)

	labelDoubleHistSlice := pdata.NewHistogramDataPointSlice()
	labelDoubleHistPoint := labelDoubleHistSlice.AppendEmpty()
	labelDoubleHistPoint.SetCount(10)
	labelDoubleHistPoint.SetSum(101.0)
	labelDoubleHistPoint.SetTimestamp(testTimestamp)
	labelDoubleHistPoint.Attributes().InsertString("labelKey", "labelValue")

	zeroHistogramSlice := pdata.NewHistogramDataPointSlice()
	zeroHistogramDataPoint := zeroHistogramSlice.AppendEmpty()
	zeroHistogramDataPoint.SetCount(0)
	zeroHistogramDataPoint.SetSum(0)
	zeroHistogramDataPoint.SetTimestamp(testTimestamp)

	type args struct {
		name string
		data pdata.HistogramDataPointSlice
		tags dimensions.NormalizedDimensionList
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
				tags: dimensions.NewNormalizedDimensionList(),
			},
			want: []string{"my_double_hist gauge,min=10.1,max=10.1,sum=101,count=10 1626438600000"},
		},
		{
			name: "Serialize double histogram data points with tags",
			args: args{
				name: "my_double_hist_with_tags",
				data: doubleHistSlice,
				tags: dimensions.NewNormalizedDimensionList(dimensions.NewDimension("test_key", "testval")),
			},
			want: []string{"my_double_hist_with_tags,test_key=testval gauge,min=10.1,max=10.1,sum=101,count=10 1626438600000"},
		},
		{
			name: "Serialize double histogram data points with labels",
			args: args{
				name: "my_double_hist_with_labels",
				data: labelDoubleHistSlice,
				tags: dimensions.NewNormalizedDimensionList(),
			},
			want: []string{"my_double_hist_with_labels,labelkey=labelValue gauge,min=10.1,max=10.1,sum=101,count=10 1626438600000"},
		},
		{
			name: "Serialize zero double histogram",
			args: args{
				name: "zero_double_hist",
				data: zeroHistogramSlice,
				tags: dimensions.NewNormalizedDimensionList(),
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeHistogramMetrics("", tt.args.name, tt.args.data, tt.args.tags); !equal(got, tt.want) {
				t.Errorf("SerializeHistogramMetrics() = %v, want %v", got, tt.want)
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
