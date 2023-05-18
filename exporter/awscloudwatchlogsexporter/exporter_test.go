// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsexporter

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

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
		resource pcommon.Resource
		log      plog.LogRecord
		config   Config
		want     cwlogs.Event
		wantErr  bool
	}{
		{
			name:     "basic",
			resource: testResource(),
			log:      testLogRecord(),
			config:   Config{},
			want: cwlogs.Event{
				GeneratedTime: time.Now(),
				InputLogEvent: &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(1609719139),
					Message:   aws.String(`{"body":"hello world","severity_number":5,"severity_text":"debug","dropped_attributes_count":4,"flags":1,"trace_id":"0102030405060708090a0b0c0d0e0f10","span_id":"0102030405060708","attributes":{"key1":1,"key2":"attr2"},"resource":{"host":"abc123","node":5}}`),
				},
				LogGroupName:  "",
				LogStreamName: "",
			},
		},
		{
			name:     "no resource",
			resource: pcommon.NewResource(),
			log:      testLogRecord(),
			config:   Config{},
			want: cwlogs.Event{
				GeneratedTime: time.Now(),
				InputLogEvent: &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(1609719139),
					Message:   aws.String(`{"body":"hello world","severity_number":5,"severity_text":"debug","dropped_attributes_count":4,"flags":1,"trace_id":"0102030405060708090a0b0c0d0e0f10","span_id":"0102030405060708","attributes":{"key1":1,"key2":"attr2"}}`),
				},
				LogGroupName:  "",
				LogStreamName: "",
			},
		},
		{
			name:     "no trace",
			resource: testResource(),
			log:      testLogRecordWithoutTrace(),
			config: Config{
				LogGroupName:  "tLogGroup",
				LogStreamName: "tStreamName",
			},
			want: cwlogs.Event{
				GeneratedTime: time.Now(),
				InputLogEvent: &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(1609719139),
					Message:   aws.String(`{"body":"hello world","severity_number":5,"severity_text":"debug","dropped_attributes_count":4,"attributes":{"key1":1,"key2":"attr2"},"resource":{"host":"abc123","node":5}}`),
				},
				LogGroupName:  "tLogGroup",
				LogStreamName: "tStreamName",
			},
		},
		{
			name:     "raw",
			resource: testResource(),
			log:      testLogRecordWithoutTrace(),
			config: Config{
				LogGroupName:  "tLogGroup",
				LogStreamName: "tStreamName",
				RawLog:        true,
			},
			want: cwlogs.Event{
				GeneratedTime: time.Now(),
				InputLogEvent: &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(1609719139),
					Message:   aws.String(`hello world`),
				},
				LogGroupName:  "tLogGroup",
				LogStreamName: "tStreamName",
			},
		},
		{
			name:     "raw emf v1",
			resource: testResource(),
			log:      createPLog(`{"_aws":{"Timestamp":1574109732004,"LogGroupName":"Foo","CloudWatchMetrics":[{"Namespace":"MyApp","Dimensions":[["Operation"]],"Metrics":[{"Name":"ProcessingLatency","Unit":"Milliseconds","StorageResolution":60}]}]},"Operation":"Aggregator","ProcessingLatency":100}`),
			config: Config{
				LogGroupName:  "tLogGroup",
				LogStreamName: "tStreamName",
				RawLog:        true,
			},
			want: cwlogs.Event{
				GeneratedTime: time.Now(),
				InputLogEvent: &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(1609719139),
					Message:   aws.String(`{"_aws":{"Timestamp":1574109732004,"LogGroupName":"Foo","CloudWatchMetrics":[{"Namespace":"MyApp","Dimensions":[["Operation"]],"Metrics":[{"Name":"ProcessingLatency","Unit":"Milliseconds","StorageResolution":60}]}]},"Operation":"Aggregator","ProcessingLatency":100}`),
				},
				LogGroupName:  "Foo",
				LogStreamName: "tStreamName",
			},
		},
		{
			name:     "raw emf v1 with log stream",
			resource: testResource(),
			log:      createPLog(`{"_aws":{"Timestamp":1574109732004,"LogGroupName":"Foo","LogStreamName":"Foo","CloudWatchMetrics":[{"Namespace":"MyApp","Dimensions":[["Operation"]],"Metrics":[{"Name":"ProcessingLatency","Unit":"Milliseconds","StorageResolution":60}]}]},"Operation":"Aggregator","ProcessingLatency":100}`),
			config: Config{
				LogGroupName:  "tLogGroup",
				LogStreamName: "tStreamName",
				RawLog:        true,
			},
			want: cwlogs.Event{
				GeneratedTime: time.Now(),
				InputLogEvent: &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(1609719139),
					Message:   aws.String(`{"_aws":{"Timestamp":1574109732004,"LogGroupName":"Foo","LogStreamName":"Foo","CloudWatchMetrics":[{"Namespace":"MyApp","Dimensions":[["Operation"]],"Metrics":[{"Name":"ProcessingLatency","Unit":"Milliseconds","StorageResolution":60}]}]},"Operation":"Aggregator","ProcessingLatency":100}`),
				},
				LogGroupName:  "Foo",
				LogStreamName: "Foo",
			},
		},
		{
			name:     "raw emf v0",
			resource: testResource(),
			log:      createPLog(`{"Timestamp":1574109732004,"log_group_name":"Foo","CloudWatchMetrics":[{"Namespace":"MyApp","Dimensions":[["Operation"]],"Metrics":[{"Name":"ProcessingLatency","Unit":"Milliseconds","StorageResolution":60}]}],"Operation":"Aggregator","ProcessingLatency":100}`),
			config: Config{
				LogGroupName:  "tLogGroup",
				LogStreamName: "tStreamName",
				RawLog:        true,
			},
			want: cwlogs.Event{
				GeneratedTime: time.Now(),
				InputLogEvent: &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(1609719139),
					Message:   aws.String(`{"Timestamp":1574109732004,"log_group_name":"Foo","CloudWatchMetrics":[{"Namespace":"MyApp","Dimensions":[["Operation"]],"Metrics":[{"Name":"ProcessingLatency","Unit":"Milliseconds","StorageResolution":60}]}],"Operation":"Aggregator","ProcessingLatency":100}`),
				},
				LogGroupName:  "Foo",
				LogStreamName: "tStreamName",
			},
		},
		{
			name:     "raw emf v0 with log stream",
			resource: testResource(),
			log:      createPLog(`{"Timestamp":1574109732004,"log_group_name":"Foo","log_stream_name":"Foo","CloudWatchMetrics":[{"Namespace":"MyApp","Dimensions":[["Operation"]],"Metrics":[{"Name":"ProcessingLatency","Unit":"Milliseconds","StorageResolution":60}]}],"Operation":"Aggregator","ProcessingLatency":100}`),
			config: Config{
				LogGroupName:  "tLogGroup",
				LogStreamName: "tStreamName",
				RawLog:        true,
			},
			want: cwlogs.Event{
				GeneratedTime: time.Now(),
				InputLogEvent: &cloudwatchlogs.InputLogEvent{
					Timestamp: aws.Int64(1609719139),
					Message:   aws.String(`{"Timestamp":1574109732004,"log_group_name":"Foo","log_stream_name":"Foo","CloudWatchMetrics":[{"Namespace":"MyApp","Dimensions":[["Operation"]],"Metrics":[{"Name":"ProcessingLatency","Unit":"Milliseconds","StorageResolution":60}]}],"Operation":"Aggregator","ProcessingLatency":100}`),
				},
				LogGroupName:  "Foo",
				LogStreamName: "Foo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceAttrs := attrsValue(tt.resource.Attributes())
			got, err := logToCWLog(resourceAttrs, tt.log, &tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("logToCWLog() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Do not test generated time since it is time.Now()
			assert.Equal(t, tt.want.InputLogEvent, got.InputLogEvent)
			assert.Equal(t, tt.want.LogStreamName, got.LogStreamName)
			assert.Equal(t, tt.want.LogGroupName, got.LogGroupName)
		})
	}
}

func BenchmarkLogToCWLog(b *testing.B) {
	b.ReportAllocs()

	resource := testResource()
	log := testLogRecord()
	for i := 0; i < b.N; i++ {
		_, err := logToCWLog(attrsValue(resource.Attributes()), log, &Config{})
		if err != nil {
			b.Errorf("logToCWLog() failed %v", err)
			return
		}
	}
}

func testResource() pcommon.Resource {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("host", "abc123")
	resource.Attributes().PutInt("node", 5)
	return resource
}

func testLogRecord() plog.LogRecord {
	record := plog.NewLogRecord()
	record.SetSeverityNumber(5)
	record.SetSeverityText("debug")
	record.SetDroppedAttributesCount(4)
	record.Body().SetStr("hello world")
	record.Attributes().PutInt("key1", 1)
	record.Attributes().PutStr("key2", "attr2")
	record.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	record.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	record.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	record.SetTimestamp(1609719139000000)
	return record
}

func testLogRecordWithoutTrace() plog.LogRecord {
	record := plog.NewLogRecord()
	record.SetSeverityNumber(5)
	record.SetSeverityText("debug")
	record.SetDroppedAttributesCount(4)
	record.Body().SetStr("hello world")
	record.Attributes().PutInt("key1", 1)
	record.Attributes().PutStr("key2", "attr2")
	record.SetTimestamp(1609719139000000)
	return record
}

func createPLog(log string) plog.LogRecord {
	pLog := plog.NewLogRecord()
	pLog.Body().SetStr(log)
	pLog.SetTimestamp(1609719139000000)
	return pLog
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
	exp, err := newCwLogsPusher(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)
	ld := plog.NewLogs()
	r := ld.ResourceLogs().AppendEmpty()
	r.Resource().Attributes().PutStr("hello", "test")
	logRecords := r.ScopeLogs().AppendEmpty().LogRecords()
	logRecords.EnsureCapacity(5)
	logRecords.AppendEmpty()
	assert.Equal(t, 1, ld.LogRecordCount())

	logPusher := new(mockPusher)
	logPusher.On("AddLogEntry", nil).Return("").Once()
	logPusher.On("ForceFlush", nil).Return("").Twice()
	exp.pusherMap[cwlogs.PusherKey{
		LogGroupName:  expCfg.LogGroupName,
		LogStreamName: expCfg.LogStreamName,
	}] = logPusher
	require.NoError(t, exp.consumeLogs(ctx, ld))
	require.NoError(t, exp.shutdown(ctx))
}

func TestNewExporterWithoutRegionErr(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.MaxRetries = 0
	exp, err := newCwLogsExporter(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, exp)
	assert.NotNil(t, err)
}
