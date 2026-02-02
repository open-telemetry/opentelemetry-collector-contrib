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
	"unicode/utf8"

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

	type s3Content struct {
		bucketName string
		objectKey  string
		data       []byte
	}

	tests := []struct {
		name          string
		s3Event       events.S3Event
		s3MockContent s3Content
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
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "test-file.txt",
				data:       []byte("Some log in S3 object"),
			},
			unmarshaler:   customLogUnmarshaler{}.UnmarshalLogs,
			eventConsumer: &noOpLogsConsumer{},
		},
		{
			name: "URL encoded S3 object key with custom unmarshaler and custom consumer",
			s3Event: events.S3Event{
				Records: []events.S3EventRecord{
					{
						EventSource: "aws:s3",
						EventTime:   time.Unix(1764625361, 0),
						S3: events.S3Entity{
							Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
							Object: events.S3Object{
								Key:  "Test-file%2810x10%29%231.txt", // Test-file(10x10)#1.txt
								Size: 10,
							},
						},
					},
				},
			},
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "Test-file(10x10)#1.txt",
				data:       []byte("Some log in S3 object"),
			},
			unmarshaler:   customLogUnmarshaler{}.UnmarshalLogs,
			eventConsumer: &noOpLogsConsumer{},
		},
		{
			name: "Valid S3 Notification Log Event built-in unmarshaler and golden validation consumer: String logs",
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
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "test-file.txt",
				data:       []byte("Some log in S3 object"),
			},
			unmarshaler:   bytesToPlogs,
			eventConsumer: &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "s3_log_expected_string.yaml")},
		},
		{
			name: "Valid S3 Notification Log Event built-in unmarshaler and golden validation consumer: Gzipped content",
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
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "test-file.txt",
				data:       []byte("H4sIAAAAAAAAAwvOz01VyMlPV8jMUwg2VshPykpNLgEAo01BGxUAAAA="),
			},
			unmarshaler:   bytesToPlogs,
			eventConsumer: &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "s3_log_expected_gzip.yaml")},
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
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "test-file.txt",
				data:       []byte("Some log in S3 object"),
			},
			unmarshaler:   customLogUnmarshaler{error: errors.New("failed to unmarshal logs")}.UnmarshalLogs,
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
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "test-file.txt",
				data:       []byte{},
			},
			eventConsumer: &noOpLogsConsumer{},
		},
	}

	ctr := gomock.NewController(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s3Service := internal.NewMockS3Service(ctr)
			s3Service.EXPECT().
				ReadObject(gomock.Any(), test.s3MockContent.bucketName, test.s3MockContent.objectKey).
				Return(test.s3MockContent.data, nil).
				AnyTimes()

			// Wrap the consumer to match the new s3EventConsumerFunc signature
			logsConsumer := func(ctx context.Context, event events.S3EventRecord, logs plog.Logs) error {
				enrichS3Logs(logs, event)
				return test.eventConsumer.ConsumeLogs(ctx, logs)
			}

			handler := newS3Handler(s3Service, zap.NewNop(), test.unmarshaler, logsConsumer)

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
	logsConsumer := func(ctx context.Context, event events.S3EventRecord, logs plog.Logs) error {
		enrichS3Logs(logs, event)
		return consumer.ConsumeLogs(ctx, logs)
	}
	handler := newS3Handler(s3Service, zap.NewNop(), customLogUnmarshaler{}.UnmarshalLogs, logsConsumer)

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

	tests := []struct {
		name            string
		eventData       string
		unmarshalerFunc func(buf []byte) (plog.Logs, error)
		eventConsumer   consumer.Logs
		expectedErr     string
	}{
		{
			name:            "Valid CloudWatch log event with built-in unmarshaler and golden validation consumer",
			eventData:       loadCompressedData(t, filepath.Join(testDataDirectory, "cloudwatch_log.json")),
			unmarshalerFunc: cwLogsToPlogs,
			eventConsumer:   &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "cloudwatch_log_expected_default.yaml")},
		},
		{
			name:            "Valid CloudWatch log event with custom unmarshaler and golden validation consumer",
			eventData:       loadCompressedData(t, filepath.Join(testDataDirectory, "cloudwatch_log.json")),
			unmarshalerFunc: customLogUnmarshaler{}.UnmarshalLogs,
			eventConsumer:   &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "cloudwatch_log_expected_custom.yaml")},
		},
		{
			name:            "Invalid CloudWatch log event - invalid base64 data",
			eventData:       "#",
			unmarshalerFunc: cwLogsToPlogs,
			expectedErr:     "failed to decode data from cloudwatch logs event",
			eventConsumer:   &noOpLogsConsumer{},
		},
		{
			name:            "Invalid CloudWatch log event - invalid json data",
			eventData:       "test",
			unmarshalerFunc: cwLogsToPlogs,
			expectedErr:     "failed to decompress data from cloudwatch subscription event",
			eventConsumer:   &noOpLogsConsumer{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cwEvent := events.CloudwatchLogsEvent{
				AWSLogs: events.CloudwatchLogsRawData{
					Data: test.eventData,
				},
			}
			var lambdaEvent json.RawMessage
			lambdaEvent, err := json.Marshal(cwEvent)
			require.NoError(t, err)

			handler := newCWLogsSubscriptionHandler(test.unmarshalerFunc, test.eventConsumer.ConsumeLogs)
			err = handler.handle(t.Context(), lambdaEvent)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEnrichS3Logs(t *testing.T) {
	t.Parallel()

	// given
	logs := plog.NewLogs()

	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs()
	lr := sl.AppendEmpty().LogRecords()
	lr.AppendEmpty()

	observedTimestamp := time.UnixMilli(1765574662915)
	expectedTimestamp := pcommon.NewTimestampFromTime(observedTimestamp)

	s3Record := events.S3EventRecord{
		AWSRegion: "us-east-1",
		EventTime: observedTimestamp,
		S3: events.S3Entity{
			SchemaVersion: "",
			Bucket: events.S3Bucket{
				Name: "bucket-name",
			},
			Object: events.S3Object{
				Key: "object-key",
			},
		},
	}

	// when
	enrichS3Logs(logs, s3Record)

	// then
	for _, resource := range logs.ResourceLogs().All() {
		resourceAttrs := resource.Resource().Attributes()

		v, b := resourceAttrs.Get("cloud.provider")
		require.True(t, b)
		require.Equal(t, "aws", v.AsString())

		v, b = resourceAttrs.Get("cloud.region")
		require.True(t, b)
		require.Equal(t, "us-east-1", v.AsString())

		v, b = resourceAttrs.Get("aws.s3.bucket")
		require.True(t, b)
		require.Equal(t, "bucket-name", v.AsString())

		v, b = resourceAttrs.Get("aws.s3.key")
		require.True(t, b)
		require.Equal(t, "object-key", v.AsString())

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
			logsConsumer := func(_ context.Context, _ events.S3EventRecord, _ plog.Logs) error {
				return test.consumerErr
			}

			handler := newS3Handler(s3Service, zap.NewNop(), customLogUnmarshaler{}.UnmarshalLogs, logsConsumer)

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
	// Uncomment below to update the source files
	// golden.WriteLogsToFile(l.logsExpectedPath, logs)
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

type customLogUnmarshaler struct {
	error error
}

func (customLogUnmarshaler) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m customLogUnmarshaler) UnmarshalLogs(data []byte) (plog.Logs, error) {
	if m.error != nil {
		return plog.Logs{}, m.error
	}

	// perform minimal unmarshaling for validations
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("customLogUnmarshaler")

	lr := sl.LogRecords().AppendEmpty()

	if utf8.Valid(data) {
		lr.Body().SetStr(string(data))
	} else {
		lr.Body().SetEmptyBytes().FromRaw(data)
	}
	return logs, nil
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
