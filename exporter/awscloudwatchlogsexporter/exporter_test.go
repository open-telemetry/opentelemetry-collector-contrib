// Copyright 2020, OpenTelemetry Authors
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

package awscloudwatchlogsexporter

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"
)

type mockPusher struct {
	mock.Mock
}

func (p *mockPusher) AddLogEntry(logEvent *cwlogs.Event) error {
	args := p.Called(nil)
	errorStr := args.String(0)
	if errorStr != "" {
		return awserr.NewRequestFailure(nil, 400, "").(error)
	}
	return nil
}

func (p *mockPusher) ForceFlush() error {
	args := p.Called(nil)
	errorStr := args.String(0)
	if errorStr != "" {
		return awserr.NewRequestFailure(nil, 400, "").(error)
	}
	return nil
}

func TestLogToCWLog(t *testing.T) {
	tests := []struct {
		name     string
		resource pdata.Resource
		log      pdata.LogRecord
		want     *cloudwatchlogs.InputLogEvent
		wantErr  bool
	}{
		{
			name:     "basic",
			resource: testResource(),
			log:      testLogRecord(),
			want: &cloudwatchlogs.InputLogEvent{
				Timestamp: aws.Int64(1609719139),
				Message:   aws.String(`{"body":"hello world","severity_number":5,"severity_text":"debug","dropped_attributes_count":4,"flags":255,"trace_id":"0102030405060708090a0b0c0d0e0f10","span_id":"0102030405060708","attributes":{"key1":1,"key2":"attr2"},"resource":{"host":"abc123","node":5}}`),
			},
		},
		{
			name:     "no resource",
			resource: pdata.NewResource(),
			log:      testLogRecord(),
			want: &cloudwatchlogs.InputLogEvent{
				Timestamp: aws.Int64(1609719139),
				Message:   aws.String(`{"body":"hello world","severity_number":5,"severity_text":"debug","dropped_attributes_count":4,"flags":255,"trace_id":"0102030405060708090a0b0c0d0e0f10","span_id":"0102030405060708","attributes":{"key1":1,"key2":"attr2"}}`),
			},
		},
		{
			name:     "no trace",
			resource: testResource(),
			log:      testLogRecordWithoutTrace(),
			want: &cloudwatchlogs.InputLogEvent{
				Timestamp: aws.Int64(1609719139),
				Message:   aws.String(`{"body":"hello world","severity_number":5,"severity_text":"debug","dropped_attributes_count":4,"attributes":{"key1":1,"key2":"attr2"},"resource":{"host":"abc123","node":5}}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceAttrs := attrsValue(tt.resource.Attributes())
			got, err := logToCWLog(resourceAttrs, tt.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("logToCWLog() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func BenchmarkLogToCWLog(b *testing.B) {
	b.ReportAllocs()

	resource := testResource()
	log := testLogRecord()
	for i := 0; i < b.N; i++ {
		logToCWLog(attrsValue(resource.Attributes()), log)
	}
}

func testResource() pdata.Resource {
	resource := pdata.NewResource()
	resource.Attributes().InsertString("host", "abc123")
	resource.Attributes().InsertInt("node", 5)
	return resource
}

func testLogRecord() pdata.LogRecord {
	record := pdata.NewLogRecord()
	record.SetSeverityNumber(5)
	record.SetSeverityText("debug")
	record.SetDroppedAttributesCount(4)
	record.Body().SetStringVal("hello world")
	record.Attributes().InsertInt("key1", 1)
	record.Attributes().InsertString("key2", "attr2")
	record.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	record.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	record.SetFlags(255)
	record.SetTimestamp(1609719139000000)
	return record
}

func testLogRecordWithoutTrace() pdata.LogRecord {
	record := pdata.NewLogRecord()
	record.SetSeverityNumber(5)
	record.SetSeverityText("debug")
	record.SetDroppedAttributesCount(4)
	record.Body().SetStringVal("hello world")
	record.Attributes().InsertInt("key1", 1)
	record.Attributes().InsertString("key2", "attr2")
	record.SetTimestamp(1609719139000000)
	return record
}

func TestAttrValue(t *testing.T) {
	tests := []struct {
		name    string
		builder func() pdata.Value
		want    interface{}
	}{
		{
			name: "null",
			builder: func() pdata.Value {
				return pdata.NewValueEmpty()
			},
			want: nil,
		},
		{
			name: "bool",
			builder: func() pdata.Value {
				return pdata.NewValueBool(true)
			},
			want: true,
		},
		{
			name: "int",
			builder: func() pdata.Value {
				return pdata.NewValueInt(5)
			},
			want: int64(5),
		},
		{
			name: "double",
			builder: func() pdata.Value {
				return pdata.NewValueDouble(6.7)
			},
			want: float64(6.7),
		},
		{
			name: "map",
			builder: func() pdata.Value {
				mAttr := pdata.NewValueMap()
				m := mAttr.MapVal()
				m.InsertString("key1", "value1")
				m.InsertNull("key2")
				m.InsertBool("key3", true)
				m.InsertInt("key4", 4)
				m.InsertDouble("key5", 5.6)
				return mAttr
			},
			want: map[string]interface{}{
				"key1": "value1",
				"key2": nil,
				"key3": true,
				"key4": int64(4),
				"key5": float64(5.6),
			},
		},
		{
			name: "array",
			builder: func() pdata.Value {
				arrAttr := pdata.NewValueSlice()
				arr := arrAttr.SliceVal()
				for _, av := range []pdata.Value{
					pdata.NewValueDouble(1.2),
					pdata.NewValueDouble(1.6),
					pdata.NewValueBool(true),
					pdata.NewValueString("hello"),
					pdata.NewValueEmpty(),
				} {
					tgt := arr.AppendEmpty()
					av.CopyTo(tgt)
				}
				return arrAttr
			},
			want: []interface{}{1.2, 1.6, true, "hello", nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attrValue(tt.builder())
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConsumeLogs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.LogGroupName = "testGroup"
	expCfg.LogStreamName = "testStream"
	expCfg.MaxRetries = 0
	exp, err := newCwLogsPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)
	ld := pdata.NewLogs()
	r := ld.ResourceLogs().AppendEmpty()
	r.Resource().Attributes().UpsertString("hello", "test")
	logRecords := r.ScopeLogs().AppendEmpty().LogRecords()
	logRecords.EnsureCapacity(5)
	logRecords.AppendEmpty()
	assert.Equal(t, 1, ld.LogRecordCount())

	logPusher := new(mockPusher)
	logPusher.On("AddLogEntry", nil).Return("").Once()
	logPusher.On("ForceFlush", nil).Return("").Twice()
	exp.(*exporter).pusher = logPusher
	require.NoError(t, exp.(*exporter).ConsumeLogs(ctx, ld))
	require.NoError(t, exp.Shutdown(ctx))
}

func TestNewExporterWithoutRegionErr(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.MaxRetries = 0
	exp, err := newCwLogsExporter(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, exp)
	assert.NotNil(t, err)
}
