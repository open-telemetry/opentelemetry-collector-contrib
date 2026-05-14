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
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
)

const testDataDirectory = "testdata"

// fixedLogsDecoder wraps a single LogsDecoderFactory in the getDecoder signature
// expected by newS3LogsHandler. Helper for tests using a fixed decoder.
func fixedLogsDecoder(factory encoding.LogsDecoderFactory) func(string) (encoding.LogsDecoderFactory, string, error) {
	return func(string) (encoding.LogsDecoderFactory, string, error) {
		return factory, "", nil
	}
}

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
		extension     encoding.LogsDecoderFactory
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
			extension:     &customLogUnmarshaler{},
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
			extension:     &customLogUnmarshaler{},
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
			extension:     internal.NewDefaultS3LogsDecoder(),
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
				data:       compressData(t, []byte("Logs in Gzip S3 object")),
			},
			extension:     internal.NewDefaultS3LogsDecoder(),
			eventConsumer: &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "s3_log_expected_gzip.yaml")},
		},
		{
			name: "Invalid S3 Notification Log Event",
			s3Event: events.S3Event{
				Records: []events.S3EventRecord{},
			},
			extension:     &customLogUnmarshaler{},
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
			extension:     &customLogUnmarshaler{error: errors.New("failed to unmarshal logs")},
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
			extension:     &customLogUnmarshaler{},
			eventConsumer: &noOpLogsConsumer{},
		},
	}

	ctr := gomock.NewController(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s3Service := internal.NewMockS3Service(ctr)
			s3Service.EXPECT().
				GetReader(gomock.Any(), test.s3MockContent.bucketName, test.s3MockContent.objectKey).
				Return(io.NopCloser(bytes.NewReader(test.s3MockContent.data)), nil).
				AnyTimes()

			handler := newS3LogsHandler(s3Service, zap.NewNop(), fixedLogsDecoder(test.extension), test.eventConsumer)

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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marshal, err := json.Marshal(test.input)
			require.NoError(t, err)

			event, err := parseS3Event(marshal)

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
		name          string
		eventData     string
		extension     encoding.LogsDecoderFactory
		eventConsumer consumer.Logs
		expectedErr   string
	}{
		{
			name:          "Valid CloudWatch log event with built-in unmarshaler and golden validation consumer",
			eventData:     loadCompressedData(t, filepath.Join(testDataDirectory, "cloudwatch_log.json")),
			extension:     internal.NewDefaultCWLogsDecoder(),
			eventConsumer: &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "cloudwatch_log_expected_default.yaml")},
		},
		{
			name:          "Valid CloudWatch log event with custom unmarshaler and golden validation consumer",
			eventData:     loadCompressedData(t, filepath.Join(testDataDirectory, "cloudwatch_log.json")),
			extension:     &customLogUnmarshaler{},
			eventConsumer: &logConsumerWithGoldenValidation{logsExpectedPath: filepath.Join(testDataDirectory, "cloudwatch_log_expected_custom.yaml")},
		},
		{
			name:          "Invalid CloudWatch log event - invalid base64 data",
			eventData:     "#",
			extension:     internal.NewDefaultCWLogsDecoder(),
			expectedErr:   "failed to decode data from cloudwatch logs event",
			eventConsumer: &noOpLogsConsumer{},
		},
		{
			name:          "Invalid CloudWatch log event - invalid json data",
			eventData:     "test",
			extension:     internal.NewDefaultCWLogsDecoder(),
			expectedErr:   "failed to decompress data from cloudwatch subscription event",
			eventConsumer: &noOpLogsConsumer{},
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

			handler := newCWLogsSubscriptionHandler(test.extension, test.eventConsumer)

			err = handler.handle(t.Context(), lambdaEvent)
			if test.expectedErr != "" {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEnrichments(t *testing.T) {
	t.Parallel()

	observedTimestamp := time.UnixMilli(1765574662915)
	expectedTimestamp := pcommon.NewTimestampFromTime(observedTimestamp)

	s3Record := events.S3EventRecord{
		AWSRegion: "us-east-1",
		EventTime: observedTimestamp,
		S3: events.S3Entity{
			SchemaVersion: "",
			Bucket: events.S3Bucket{
				Name: "bucket-name",
				Arn:  "arn:aws:s3:::bucket-name",
			},
			Object: events.S3Object{
				Key: "object-key",
			},
		},
	}

	// given
	logs := plog.NewLogs()

	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs()
	lr := sl.AppendEmpty().LogRecords()
	lr.AppendEmpty()

	// when
	enrichS3Logs(logs, s3Record)
	enrichedCtx := getEnrichedContext(t.Context(), s3Record)

	t.Run("Validate log enrichment", func(t *testing.T) {
		for _, resource := range logs.ResourceLogs().All() {
			resourceAttrs := resource.Resource().Attributes()

			v, b := resourceAttrs.Get("cloud.provider")
			require.True(t, b)
			require.Equal(t, "aws", v.AsString())

			v, b = resourceAttrs.Get("cloud.region")
			require.True(t, b)
			require.Equal(t, "us-east-1", v.AsString())

			v, b = resourceAttrs.Get("aws.s3.bucket.name")
			require.True(t, b)
			require.Equal(t, "bucket-name", v.AsString())

			v, b = resourceAttrs.Get("aws.s3.bucket.arn")
			require.True(t, b)
			require.Equal(t, "arn:aws:s3:::bucket-name", v.AsString())

			v, b = resourceAttrs.Get("aws.s3.key")
			require.True(t, b)
			require.Equal(t, "object-key", v.AsString())

			for _, scope := range resource.ScopeLogs().All() {
				for _, logRecord := range scope.LogRecords().All() {
					require.Equal(t, expectedTimestamp, logRecord.ObservedTimestamp())
				}
			}
		}
	})

	t.Run("Validate context enrichment", func(t *testing.T) {
		info := client.FromContext(enrichedCtx)
		metadata := info.Metadata

		require.Equal(t, "us-east-1", metadata.Get("cloud.region")[0])
		require.Equal(t, "bucket-name", metadata.Get("aws.s3.bucket.name")[0])
		require.Equal(t, "arn:aws:s3:::bucket-name", metadata.Get("aws.s3.bucket.arn")[0])
		require.Equal(t, "object-key", metadata.Get("aws.s3.key")[0])
	})
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
			s3Service.EXPECT().GetReader(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(io.NopCloser(bytes.NewReader([]byte("object content"))), nil).
				Times(1)

			handler := newS3LogsHandler(s3Service, zap.NewNop(), fixedLogsDecoder(&customLogUnmarshaler{}), &noOpLogsConsumer{err: test.consumerErr})

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

func (*customLogUnmarshaler) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*customLogUnmarshaler) Shutdown(_ context.Context) error {
	return nil
}

func (*customLogUnmarshaler) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (m *customLogUnmarshaler) UnmarshalLogs(data []byte) (plog.Logs, error) {
	if m.error != nil {
		return plog.Logs{}, m.error
	}

	// perform minimal unmarshaling for validations
	return m.makeLog(data), nil
}

func (m *customLogUnmarshaler) NewLogsDecoder(reader io.Reader, _ ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	if m.error != nil {
		return nil, m.error
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	isEOF := false
	return xstreamencoding.NewLogsDecoderAdapter(
			func() (plog.Logs, error) {
				if isEOF {
					return plog.Logs{}, io.EOF
				}

				isEOF = true
				return m.makeLog(data), nil
			}, func() int64 {
				return 0
			}),
		nil
}

func (*customLogUnmarshaler) makeLog(data []byte) plog.Logs {
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
	return logs
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

func TestMultiFormatS3LogsHandler(t *testing.T) {
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
		encodings     []S3Encoding
		decoders      map[string]encoding.LogsDecoderFactory
		expectedErr   string
	}{
		{
			name: "routes VPC flow log to correct decoder",
			s3Event: events.S3Event{Records: []events.S3EventRecord{{
				EventSource: "aws:s3",
				AWSRegion:   "us-east-1",
				EventTime:   time.Unix(1764625361, 0),
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
					Object: events.S3Object{
						Key:           "AWSLogs/123/vpcflowlogs/us-east-1/file.log.gz",
						URLDecodedKey: "AWSLogs/123/vpcflowlogs/us-east-1/file.log.gz",
						Size:          10,
					},
				},
			}}},
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "AWSLogs/123/vpcflowlogs/us-east-1/file.log.gz",
				data:       []byte("vpc flow log data"),
			},
			encodings: []S3Encoding{
				{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
				{Name: "cloudtrail", Encoding: "awslogs_encoding/ct"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow":    &customLogUnmarshaler{},
				"cloudtrail": &customLogUnmarshaler{},
			},
		},
		{
			name: "routes CloudTrail to correct decoder",
			s3Event: events.S3Event{Records: []events.S3EventRecord{{
				EventSource: "aws:s3",
				AWSRegion:   "us-east-1",
				EventTime:   time.Unix(1764625361, 0),
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
					Object: events.S3Object{
						Key:           "AWSLogs/123/CloudTrail/us-east-1/file.json.gz",
						URLDecodedKey: "AWSLogs/123/CloudTrail/us-east-1/file.json.gz",
						Size:          10,
					},
				},
			}}},
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "AWSLogs/123/CloudTrail/us-east-1/file.json.gz",
				data:       []byte("cloudtrail data"),
			},
			encodings: []S3Encoding{
				{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
				{Name: "cloudtrail", Encoding: "awslogs_encoding/ct"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow":    &customLogUnmarshaler{},
				"cloudtrail": &customLogUnmarshaler{},
			},
		},
		{
			name: "no matching pattern returns error",
			s3Event: events.S3Event{Records: []events.S3EventRecord{{
				EventSource: "aws:s3",
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
					Object: events.S3Object{
						Key:           "unknown/path/file.log",
						URLDecodedKey: "unknown/path/file.log",
						Size:          10,
					},
				},
			}}},
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "unknown/path/file.log",
				data:       []byte("some data"),
			},
			encodings: []S3Encoding{{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"}},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow": &customLogUnmarshaler{},
			},
			expectedErr: "no encoding matches S3 object key",
		},
		{
			name: "catch-all routes unmatched object to default decoder",
			s3Event: events.S3Event{Records: []events.S3EventRecord{{
				EventSource: "aws:s3",
				AWSRegion:   "us-east-1",
				EventTime:   time.Unix(1764625361, 0),
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
					Object: events.S3Object{
						Key:           "random/path/file.log",
						URLDecodedKey: "random/path/file.log",
						Size:          10,
					},
				},
			}}},
			s3MockContent: s3Content{
				bucketName: "test-bucket",
				objectKey:  "random/path/file.log",
				data:       []byte("random data"),
			},
			encodings: []S3Encoding{
				{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"},
				{Name: "catchall", PathPattern: "*"},
			},
			decoders: map[string]encoding.LogsDecoderFactory{
				"vpcflow":  &customLogUnmarshaler{},
				"catchall": internal.NewDefaultS3LogsDecoder(),
			},
		},
		{
			name: "skips empty object",
			s3Event: events.S3Event{Records: []events.S3EventRecord{{
				EventSource: "aws:s3",
				S3: events.S3Entity{
					Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
					Object: events.S3Object{
						Key:           "AWSLogs/123/vpcflowlogs/file.log",
						URLDecodedKey: "AWSLogs/123/vpcflowlogs/file.log",
						Size:          0,
					},
				},
			}}},
			s3MockContent: s3Content{bucketName: "test-bucket", objectKey: "AWSLogs/123/vpcflowlogs/file.log"},
			encodings:     []S3Encoding{{Name: "vpcflow", Encoding: "awslogs_encoding/vpc"}},
			decoders:      map[string]encoding.LogsDecoderFactory{"vpcflow": &customLogUnmarshaler{}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctr := gomock.NewController(t)
			s3Service := internal.NewMockS3Service(ctr)
			s3Service.EXPECT().
				GetReader(gomock.Any(), test.s3MockContent.bucketName, test.s3MockContent.objectKey).
				Return(io.NopCloser(bytes.NewReader(test.s3MockContent.data)), nil).
				AnyTimes()

			router := newLogsDecoderRouter(test.encodings, test.decoders)
			handler := newS3LogsHandler(s3Service, zap.NewNop(), router.GetDecoder, &noOpLogsConsumer{})

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
