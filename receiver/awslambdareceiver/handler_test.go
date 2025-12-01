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
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

const testDataDirectory = "testdata"

func TestProcessLambdaEvent_S3LogNotification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		s3Event       events.S3Event
		bucketData    []byte
		unmarshaler   func(buf []byte) (plog.Logs, error)
		eventConsumer consumer.Logs
		expectedErr   string
	}{
		{
			name: "Valid S3 Notification Log Event with custom unmarshaler and custom consumer",
			s3Event: events.S3Event{
				Records: []events.S3EventRecord{
					{
						EventSource: "aws:s3",
						EventTime:   time.Unix(1764625361, 0),
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
			bucketData:    []byte("Some log in S3 object"),
			unmarshaler:   mockS3LogUnmarshaler{}.UnmarshalLogs,
			eventConsumer: &noOpLogsConsumer{},
		},
		{
			name: "Valid S3 Notification Log Event built-in unmarshaler and golden validation consumer",
			s3Event: events.S3Event{
				Records: []events.S3EventRecord{
					{
						EventSource: "aws:s3",
						AWSRegion:   "us-east-1",
						EventTime:   time.Unix(1764625361, 0),
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
			bucketData:    []byte("Some log in S3 object"),
			unmarshaler:   nil,
			eventConsumer: &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "s3_log_expected.yaml")},
		},
		{
			name: "Invalid S3 Notification Log Event",
			s3Event: events.S3Event{
				Records: []events.S3EventRecord{},
			},
			eventConsumer: &noOpLogsConsumer{},
			expectedErr:   "s3 event notification should contain one record instead of 0",
		},
		{
			name: "Error from unmarshaler is propagated to handler",
			s3Event: events.S3Event{
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
			unmarshaler:   mockS3LogUnmarshaler{error: errors.New("failed to unmarshal logs")}.UnmarshalLogs,
			eventConsumer: &noOpLogsConsumer{},
			expectedErr:   "failed to unmarshal logs",
		},
		{
			name: "Handling empty S3 object",
			s3Event: events.S3Event{
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
			bucketData:    []byte{},
			eventConsumer: &noOpLogsConsumer{},
		},
	}

	ctr := gomock.NewController(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s3Service := internal.NewMockS3Service(ctr)
			s3Service.EXPECT().ReadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return(test.bucketData, nil).AnyTimes()

			// Wrap the consumer to match the new s3EventConsumerFunc signature
			logsConsumer := func(ctx context.Context, time time.Time, logs plog.Logs) error {
				setObservedTimestampForAllLogs(logs, time)
				return test.eventConsumer.ConsumeLogs(ctx, logs)
			}

			handler := newS3Handler(s3Service, zap.NewNop(), test.unmarshaler, logsConsumer, plog.Logs{})

			var event json.RawMessage
			event, err := json.Marshal(test.s3Event)
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
	s3Service.EXPECT().ReadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return([]byte("S3 content"), nil).AnyTimes()

	var consumer noOpLogsConsumer
	// Wrap the consumer to match the new s3EventConsumerFunc signature
	logsConsumer := func(ctx context.Context, time time.Time, logs plog.Logs) error {
		setObservedTimestampForAllLogs(logs, time)
		return consumer.ConsumeLogs(ctx, logs)
	}
	handler := newS3Handler(s3Service, zap.NewNop(), mockS3LogUnmarshaler{}.UnmarshalLogs, logsConsumer, plog.Logs{})

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

func TestHandleCloudwatchLogEvent(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		eventData     string
		expectedErr   string
		eventConsumer consumer.Logs
	}{
		"valid_cloudwatch_log_event": {
			eventData:     loadCompressedData(t, filepath.Join(testDataDirectory, "cloudwatch_log.json")),
			eventConsumer: &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "cloudwatch_log_expected.yaml")},
		},
		"invalid_base64_data": {
			eventData:     "#",
			expectedErr:   "failed to decode data from cloudwatch logs event",
			eventConsumer: &noOpLogsConsumer{},
		},
		"invalid_cloudwatch_log_data": {
			eventData:     "test",
			expectedErr:   "failed to decompress data from cloudwatch subscription event",
			eventConsumer: &noOpLogsConsumer{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cwEvent := events.CloudwatchLogsEvent{
				AWSLogs: events.CloudwatchLogsRawData{
					Data: test.eventData,
				},
			}
			var lambdaEvent json.RawMessage
			lambdaEvent, err := json.Marshal(cwEvent)
			require.NoError(t, err)

			// CW Logs subscription handler with default unmarshaler
			handler := newCWLogsSubscriptionHandler(zap.NewNop(), nil, test.eventConsumer.ConsumeLogs)
			err = handler.handle(t.Context(), lambdaEvent)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
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
	attr.PutStr("cloud.provider", "aws")

	// Add ScopeLogs
	scopeLogs := plog.NewScopeLogs()
	scopeLogs.Scope().SetName("test")
	recordLog := plog.NewLogRecord()

	// Add record attributes
	recordLog.Attributes().PutStr("client.address", "0.0.0.0")
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

func TestConsumerErrorHandling(t *testing.T) {
	t.Parallel()

	mockEvent := events.S3Event{
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
	}

	tests := map[string]struct {
		consumerErr     error
		expectRetryable bool
		expectPermanent bool
	}{
		"plain_error": {
			consumerErr:     errors.New("plain error"),
			expectRetryable: true,
			expectPermanent: false,
		},
		"permanent_error": {
			consumerErr:     consumererror.NewPermanent(errors.New("permanent error")),
			expectRetryable: false,
			expectPermanent: true,
		},
		"retryable_error": {
			consumerErr:     consumererror.NewRetryableError(errors.New("already retryable")),
			expectRetryable: true,
			expectPermanent: false,
		},
	}

	ctr := gomock.NewController(t)

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s3Service := internal.NewMockS3Service(ctr)
			s3Service.EXPECT().ReadObject(gomock.Any(), gomock.Any(), gomock.Any()).Return([]byte("object content"), nil).Times(1)

			// Consumer that returns the test error
			logsConsumer := func(_ context.Context, _ time.Time, _ plog.Logs) error {
				return test.consumerErr
			}

			handler := newS3Handler(s3Service, zap.NewNop(), mockS3LogUnmarshaler{}.UnmarshalLogs, logsConsumer, plog.Logs{})

			event, err := json.Marshal(mockEvent)
			require.NoError(t, err)

			resultErr := handler.handle(t.Context(), event)
			require.Error(t, resultErr)

			// Check if the error is permanent
			if test.expectPermanent {
				require.True(t, consumererror.IsPermanent(resultErr), "expected permanent error")
			} else {
				require.False(t, consumererror.IsPermanent(resultErr), "expected non-permanent error")
			}
		})
	}
}

type logConsumerWithGoldenValidation struct {
	logsExpectedPath string
}

func (logConsumerWithGoldenValidation) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (l logConsumerWithGoldenValidation) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	expectedLogs, err := golden.ReadLogs(l.logsExpectedPath)
	if err != nil {
		return err
	}

	return plogtest.CompareLogs(expectedLogs, logs)
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

type mockS3LogUnmarshaler struct {
	error error
}

func (mockS3LogUnmarshaler) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m mockS3LogUnmarshaler) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	if m.error != nil {
		return plog.Logs{}, m.error
	}

	return plog.NewLogs(), nil
}

type mockHandlerProvider struct {
	handler lambdaEventHandler
}

func (m mockHandlerProvider) getHandler(_ eventType) (lambdaEventHandler, error) {
	return m.handler, nil
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

func loadCompressedData(t *testing.T, file string) string {
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
