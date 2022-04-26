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

package tracking

import (
	"bytes"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestMetricIdentity_Write(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().InsertBool("resource", true)

	il := pcommon.NewInstrumentationScope()
	il.SetName("ilm_name")
	il.SetVersion("ilm_version")

	attributes := pcommon.NewMap()
	attributes.InsertString("label", "value")
	type fields struct {
		Resource               pcommon.Resource
		InstrumentationLibrary pcommon.InstrumentationScope
		MetricDataType         pmetric.MetricDataType
		MetricIsMonotonic      bool
		MetricName             string
		MetricUnit             string
		StartTimestamp         pcommon.Timestamp
		Attributes             pcommon.Map
		MetricValueType        pmetric.NumberDataPointValueType
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "all present",
			fields: fields{
				Resource:               resource,
				InstrumentationLibrary: il,
				Attributes:             attributes,
				MetricName:             "m_name",
				MetricUnit:             "m_unit",
			},
			want: []string{"A" + SEPSTR + "A", "resource:true", "ilm_name", "ilm_version", "label:value", "N", "0", "m_name", "m_unit"},
		},
		{
			name: "value and data type",
			fields: fields{
				Resource:               resource,
				InstrumentationLibrary: il,
				Attributes:             attributes,
				MetricDataType:         pmetric.MetricDataTypeSum,
				MetricValueType:        pmetric.NumberDataPointValueTypeInt,
				MetricIsMonotonic:      true,
			},
			want: []string{"C" + SEPSTR + "B", "Y"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &MetricIdentity{
				Resource:               tt.fields.Resource,
				InstrumentationLibrary: tt.fields.InstrumentationLibrary,
				MetricDataType:         tt.fields.MetricDataType,
				MetricIsMonotonic:      tt.fields.MetricIsMonotonic,
				MetricName:             tt.fields.MetricName,
				MetricUnit:             tt.fields.MetricUnit,
				StartTimestamp:         tt.fields.StartTimestamp,
				Attributes:             tt.fields.Attributes,
				MetricValueType:        tt.fields.MetricValueType,
			}
			b := &bytes.Buffer{}
			mi.Write(b)
			got := b.String()
			for _, want := range tt.want {
				if !strings.Contains(got, SEPSTR+want+SEPSTR) && !strings.HasSuffix(got, SEPSTR+want) && !strings.HasPrefix(got, want+SEPSTR) {
					t.Errorf("MetricIdentity.Write() = %v, want %v", got, want)
				}
			}
		})
	}
}

func TestMetricIdentity_IsFloatVal(t *testing.T) {
	type fields struct {
		MetricValueType pmetric.NumberDataPointValueType
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "float",
			fields: fields{
				MetricValueType: pmetric.NumberDataPointValueTypeDouble,
			},
			want: true,
		},
		{
			name: "int",
			fields: fields{
				MetricValueType: pmetric.NumberDataPointValueTypeInt,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &MetricIdentity{
				Resource:               pcommon.NewResource(),
				InstrumentationLibrary: pcommon.NewInstrumentationScope(),
				Attributes:             pcommon.NewMap(),
				MetricDataType:         pmetric.MetricDataTypeSum,
				MetricValueType:        tt.fields.MetricValueType,
			}
			if got := mi.IsFloatVal(); got != tt.want {
				t.Errorf("MetricIdentity.IsFloatVal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetricIdentity_IsSupportedMetricType(t *testing.T) {
	type fields struct {
		MetricDataType pmetric.MetricDataType
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "sum",
			fields: fields{
				MetricDataType: pmetric.MetricDataTypeSum,
			},
			want: true,
		},
		{
			name: "histogram",
			fields: fields{
				MetricDataType: pmetric.MetricDataTypeHistogram,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &MetricIdentity{
				Resource:               pcommon.NewResource(),
				InstrumentationLibrary: pcommon.NewInstrumentationScope(),
				Attributes:             pcommon.NewMap(),
				MetricDataType:         tt.fields.MetricDataType,
			}
			if got := mi.IsSupportedMetricType(); got != tt.want {
				t.Errorf("MetricIdentity.IsSupportedMetricType() = %v, want %v", got, tt.want)
			}
		})
	}
}
