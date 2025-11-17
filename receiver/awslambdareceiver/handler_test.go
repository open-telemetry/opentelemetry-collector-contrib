// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awslambdareceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

func TestProcessLambdaEvent_S3Notification(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		mockEvent     events.S3Event
		mockContent   []byte
		expectedErr   string
		eventConsumer consumer.Logs
	}{
		"valid_s3_notification_log_event": {
			mockEvent: events.S3Event{
				Records: []events.S3EventRecord{
					{
						EventSource: "aws:s3",
						S3: events.S3Entity{
							Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
							Object: events.S3Object{
								Key:  "test-file.txt",
								Size: 10,
							},
						},
					},
				},
			},
			mockContent:   []byte(mockContent),
			eventConsumer: &noOpLogsConsumer{},
		},
		"invalid_s3_notification_log_event": {
			mockEvent: events.S3Event{
				Records: []events.S3EventRecord{},
			},
			mockContent:   []byte(mockContent),
			expectedErr:   "s3 event notification should contain one record instead of 0",
			eventConsumer: &noOpLogsConsumer{},
		},
		"s3_notification_unmarshal_error": {
			mockEvent: events.S3Event{
				Records: []events.S3EventRecord{
					{
						EventSource: "aws:s3",
						S3: events.S3Entity{
							Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
							Object: events.S3Object{
								Key:  "test-file.txt",
								Size: 10,
							},
						},
					},
				},
			},
			mockContent:   []byte("mock log content causing unmarshaler failure"),
			expectedErr:   "failed to unmarshal logs",
			eventConsumer: &noOpLogsConsumer{},
		},
		"s3_empty_files_are_ignored": {
			mockEvent: events.S3Event{
				Records: []events.S3EventRecord{
					{
						EventSource: "aws:s3",
						S3: events.S3Entity{
							Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
							Object: events.S3Object{
								Key:  "test-file.txt",
								Size: 0, // size zero is considered to be empty object
							},
						},
					},
				},
			},
			mockContent:   []byte{},
			eventConsumer: &noOpLogsConsumer{},
		},
	}

	ctr := gomock.NewController(t)

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s3Service := internal.NewMockS3Service(ctr)
			s3Service.EXPECT().ReadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(test.mockContent, nil).AnyTimes()

			handler := newS3Handler(s3Service, zap.NewNop(), mockS3LogUnmarshaler{}.UnmarshalLogs, test.eventConsumer.ConsumeLogs)

			var event json.RawMessage
			event, err := json.Marshal(test.mockEvent)
			require.NoError(t, err)

			errP := handler.handle(t.Context(), event)
			if test.expectedErr != "" {
				require.ErrorContains(t, errP, test.expectedErr)
			} else {
				require.NoError(t, errP)
			}
		})
	}
}

func TestS3HandlerParseEvent(t *testing.T) {
	tests := []struct {
		name     string
		input    events.S3Event
		isError  bool
		expected events.S3EventRecord
	}{
		{
			name: "valid_event_log_event",
			input: events.S3Event{
				Records: []events.S3EventRecord{
					{
						EventSource: "aws:s3",
						S3: events.S3Entity{
							Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
							Object: events.S3Object{
								Key:           "test-file.txt",
								Size:          10,
								URLDecodedKey: "test-file.txt",
							},
						},
					},
				},
			},
			expected: events.S3EventRecord{
				EventSource: "aws:s3",
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
					Object: events.S3Object{
						Key:           "test-file.txt",
						Size:          10,
						URLDecodedKey: "test-file.txt",
					},
				},
			},
		},
		{
			name: "invalid_event_multiple_records",
			input: events.S3Event{
				Records: []events.S3EventRecord{
					{
						EventSource: "aws:s3",
						S3: events.S3Entity{
							Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
							Object: events.S3Object{
								Key:  "test-file.txt",
								Size: 10,
							},
						},
					},
					{
						EventSource: "aws:s3",
						S3: events.S3Entity{
							Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
							Object: events.S3Object{
								Key:  "test-file.txt",
								Size: 10,
							},
						},
					},
				},
			},
			isError: true,
		},
	}

	ctr := gomock.NewController(t)
	s3Service := internal.NewMockS3Service(ctr)
	s3Service.EXPECT().ReadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return([]byte(mockContent), nil).AnyTimes()

	var consumer noOpLogsConsumer
	handler := newS3Handler(s3Service, zap.NewNop(), mockS3LogUnmarshaler{}.UnmarshalLogs, consumer.ConsumeLogs)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marshal, err := json.Marshal(test.input)
			require.NoError(t, err)

			event, err := handler.parseEvent(marshal)

			if test.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expected, event)
			}
		})
	}
}

func TestSetObservedTimestampForAllLogs(t *testing.T) {
	t.Parallel()

	logs := plog.NewLogs()

	// Add ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	attr := rl.Resource().Attributes()
	attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())

	// Add ScopeLogs
	scopeLogs := plog.NewScopeLogs()
	scopeLogs.Scope().SetName("test")
	recordLog := plog.NewLogRecord()

	// Add record attributes
	recordLog.Attributes().PutStr(string(conventions.ClientAddressKey), "0.0.0.0")
	rScope := scopeLogs.LogRecords().AppendEmpty()
	recordLog.MoveTo(rScope)

	scopeLogs.MoveTo(rl.ScopeLogs().AppendEmpty())

	// Set the observed timestamp
	observedTimestamp := time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC)
	setObservedTimestampForAllLogs(logs, observedTimestamp)

	// Assert all LogRecords have the expected timestamp
	expectedTimestamp := pcommon.NewTimestampFromTime(observedTimestamp)

	for _, resource := range logs.ResourceLogs().All() {
		for _, scope := range resource.ScopeLogs().All() {
			for _, logRecord := range scope.LogRecords().All() {
				require.Equal(t, expectedTimestamp, logRecord.ObservedTimestamp())
			}
		}
	}
}

type noOpLogsConsumer struct {
	consumeCount int
	err          error
}

func (*noOpLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (n *noOpLogsConsumer) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	n.consumeCount++
	return n.err
}

type mockS3LogUnmarshaler struct{}

func (mockS3LogUnmarshaler) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (mockS3LogUnmarshaler) UnmarshalLogs(data []byte) (plog.Logs, error) {
	if string(data) == mockContent {
		return plog.NewLogs(), nil
	}
	return plog.Logs{}, errors.New("logs not in the correct format")
}

type mockPlogEventHandler struct {
	handleCount int
	event       eventType
}

func (n *mockPlogEventHandler) handlerType() eventType {
	return n.event
}

func (n *mockPlogEventHandler) handle(context.Context, json.RawMessage) error {
	n.handleCount++
	return nil
}

func getDataFromFile(t *testing.T, file string) string {
	data, err := os.ReadFile(file)
	require.NoError(t, err)

	compressed := compressData(t, data)
	return base64.StdEncoding.EncodeToString(compressed)
}

func compressData(t *testing.T, data []byte) []byte {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err := gzipWriter.Write(data)
	require.NoError(t, err)
	err = gzipWriter.Close()
	require.NoError(t, err)
	return buf.Bytes()
}
