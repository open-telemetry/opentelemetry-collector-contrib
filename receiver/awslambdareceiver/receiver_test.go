// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awslambdareceiver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal/metadata"
)

func TestCreateLogs(t *testing.T) {
	// Set Lambda environment variables required by Start()
	t.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_python3.12")

	// Create receiver using factory with S3 encoding config.
	// Note: The S3Encoding value must match the component ID used when registering the extension.

	s3Encoding := "awslogs_encoding"

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.S3.Encoding = s3Encoding
	settings := receivertest.NewNopSettings(metadata.Type)

	sink := consumertest.LogsSink{}
	receiver, err := factory.CreateLogs(
		t.Context(),
		settings,
		cfg,
		&sink,
	)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	goMock := gomock.NewController(t)
	s3Service := internal.NewMockS3Service(goMock)
	s3Provider := internal.NewMockS3Provider(goMock)
	s3Provider.EXPECT().GetService(gomock.Any()).AnyTimes().Return(s3Service, nil)

	// Test data - mock S3 file content
	testData := []byte("version account-id interface-id srcaddr dstaddr srcport dstport protocol packets bytes start end action log-status\n2 627286350134 eni-0377aa710071c557e 172.31.31.124 140.82.121.6 52718 443 6 13 3777 1751375679 ENDTIME ACCEPT OK\n")
	s3Service.EXPECT().ReadObject(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(testData, nil)

	// Register the extension with the same component ID as S3Encoding
	// This is required: the extension ID must match cfg.S3Encoding
	host := mockHost{GetFunc: func() map[component.ID]component.Component {
		return map[component.ID]component.Component{
			component.MustNewID(s3Encoding): &mockExtensionWithPLogUnmarshaler{
				Unmarshaler: unmarshalLogsFunc(func(data []byte) (plog.Logs, error) {
					require.Equal(t, string(testData), string(data))
					logs := plog.NewLogs()
					rl := logs.ResourceLogs().AppendEmpty()
					sl := rl.ScopeLogs().AppendEmpty()
					sl.LogRecords().AppendEmpty()
					return logs, nil
				}),
			},
		}
	}}

	// Initialize the handlerProvider manually (without starting Lambda runtime to avoid goroutine leaks in tests)
	awsReceiver := receiver.(*awsLambdaReceiver)
	awsReceiver.hp, err = newLogsHandler(t.Context(), cfg, settings, host, &sink, s3Provider)
	require.NoError(t, err)

	// Process an S3 event notification.
	lambdaEvent, err := json.Marshal(events.S3Event{
		Records: []events.S3EventRecord{{
			EventSource: "aws:s3",
			S3: events.S3Entity{
				Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
				Object: events.S3Object{Key: "test-file.txt", Size: int64(len(testData))},
			},
		}},
	})
	require.NoError(t, err)
	err = awsReceiver.processLambdaEvent(t.Context(), lambdaEvent)
	require.NoError(t, err)

	// Verify logs were sent to the sink
	require.NotZero(t, sink.LogRecordCount(), "Expected logs to be sent to sink")
}

func TestCreateMetrics(t *testing.T) {
	// Set Lambda environment variables required by Start()
	t.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_python3.12")

	// Custom encoder name for the test
	encoderName := "dummy_metric_encoding"

	// Create receiver using factory with a dummy encoding extension.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.S3.Encoding = encoderName
	settings := receivertest.NewNopSettings(metadata.Type)

	sink := consumertest.MetricsSink{}
	receiver, err := factory.CreateMetrics(
		t.Context(),
		settings,
		cfg,
		&sink,
	)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	goMock := gomock.NewController(t)
	s3Service := internal.NewMockS3Service(goMock)
	s3Provider := internal.NewMockS3Provider(goMock)
	s3Provider.EXPECT().GetService(gomock.Any()).AnyTimes().Return(s3Service, nil)
	s3Service.EXPECT().ReadObject(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(
		[]byte("dummy data"), nil,
	)

	host := mockHost{GetFunc: func() map[component.ID]component.Component {
		return map[component.ID]component.Component{
			component.MustNewID(encoderName): &mockExtensionWithPMetricUnmarshaler{
				Unmarshaler: unmarshalMetricsFunc(func(data []byte) (pmetric.Metrics, error) {
					require.Equal(t, "dummy data", string(data))
					metrics := pmetric.NewMetrics()
					rm := metrics.ResourceMetrics().AppendEmpty()
					ilm := rm.ScopeMetrics().AppendEmpty()
					dp := ilm.Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
					dp.SetIntValue(123)
					return metrics, nil
				}),
			},
		}
	}}

	// Initialize the handlerProvider manually (without starting Lambda runtime to avoid goroutine leaks in tests)
	awsReceiver := receiver.(*awsLambdaReceiver)
	awsReceiver.hp, err = newMetricsHandler(t.Context(), cfg, settings, host, &sink, s3Provider)
	require.NoError(t, err)

	// Process an S3 event notification.
	lambdaEvent, err := json.Marshal(events.S3Event{
		Records: []events.S3EventRecord{{
			EventSource: "aws:s3",
			S3: events.S3Entity{
				Bucket: events.S3Bucket{Name: "test-bucket", Arn: "arn:aws:s3:::test-bucket"},
				Object: events.S3Object{Key: "test-file.txt", Size: 10},
			},
		}},
	})
	require.NoError(t, err)
	err = awsReceiver.processLambdaEvent(t.Context(), lambdaEvent)
	require.NoError(t, err)

	// Verify metrics were sent to the sink
	require.NotZero(t, sink.DataPointCount(), "Expected metrics to be sent to sink")
}

func TestStartRequiresLambdaEnvironment(t *testing.T) {
	// Ensure Lambda environment variables are not set
	t.Setenv("AWS_EXECUTION_ENV", "")

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.S3.Encoding = "test_encoding"

	receiver, err := factory.CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	// Start should fail when Lambda environment variables are not set
	host := mockHost{GetFunc: func() map[component.ID]component.Component {
		return map[component.ID]component.Component{}
	}}

	err = receiver.Start(t.Context(), host)
	require.Error(t, err)
	require.Contains(t, err.Error(), "receiver must be used in an AWS Lambda environment")
}

func TestProcessLambdaEvent(t *testing.T) {
	commonCfg := Config{
		S3: sharedConfig{
			Encoding: "awslogs",
		},
	}

	commonLogger := zap.NewNop()

	tests := []struct {
		name         string
		eventType    eventType
		event        []byte
		isIgnore     bool
		isError      bool
		errorContent string
	}{
		{
			name:      "Successful S3 event",
			eventType: s3Event,
			event:     []byte(`{"Records": "S3 event content"}`),
		},
		{
			name:      "Unknown events are ignored",
			eventType: s3Event,
			event:     []byte(`{"key": "value"}`),
			isIgnore:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := &mockPlogEventHandler{}
			hpMock := &mockHandlerProvider{mockHandler}

			receiver := awsLambdaReceiver{
				cfg:    &commonCfg,
				logger: commonLogger,
				hp:     hpMock,
			}

			err := receiver.processLambdaEvent(t.Context(), tt.event)
			if tt.isError {
				require.ErrorContains(t, err, tt.errorContent)
				return
			}
			require.NoError(t, err)

			if tt.isIgnore {
				// no error & event is ignored with logging
				return
			}

			require.Equal(t, 1, mockHandler.handleCount)
		})
	}
}

func TestLoadEncodingExtension(t *testing.T) {
	tests := []struct {
		name        string
		encoding    string
		hostGetMock func() map[component.ID]component.Component
		isError     bool
	}{
		{
			name:     "success",
			encoding: "my_encoding",
			hostGetMock: func() map[component.ID]component.Component {
				id := component.NewID(component.MustNewType("my_encoding"))

				return map[component.ID]component.Component{
					id: &mockExtensionWithPLogUnmarshaler{},
				}
			},
			isError: false,
		},
		{
			name:     "success - ID with type",
			encoding: "encoder/my_encoding",
			hostGetMock: func() map[component.ID]component.Component {
				id := component.NewIDWithName(component.MustNewType("encoder"), "my_encoding")

				return map[component.ID]component.Component{
					id: &mockExtensionWithPLogUnmarshaler{},
				}
			},
			isError: false,
		},
		{
			name:     "failure - ID not found",
			encoding: "my_encoding",
			hostGetMock: func() map[component.ID]component.Component {
				// empty host components
				return map[component.ID]component.Component{}
			},
			isError: true,
		},
		{
			name:     "failure - Invalid ID format",
			encoding: "encoder-my_encoding",
			hostGetMock: func() map[component.ID]component.Component {
				// empty host components
				return map[component.ID]component.Component{}
			},
			isError: true,
		},
		{
			name:     "failure - encoder present but invalid unmarshaler type",
			encoding: "my_encoding",
			hostGetMock: func() map[component.ID]component.Component {
				id := component.NewID(component.MustNewType("my_encoding"))

				return map[component.ID]component.Component{
					// register metric unmarshaler and force failure
					id: &mockExtensionWithPMetricUnmarshaler{},
				}
			},
			isError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := mockHost{
				GetFunc: tt.hostGetMock,
			}

			ext, err := loadEncodingExtension[plog.Unmarshaler](host, tt.encoding, "logs")
			if tt.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, ext)
			}
		})
	}
}

func TestExtractFirstKey(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wants      string
		wantsError string
	}{
		{
			name:  "Simple extraction",
			input: `{"test": "test"}`,
			wants: "test",
		},
		{
			name: "Formatted single key",
			input: `   
					{
    					"test": "test"
					}`,
			wants: "test",
		},
		{
			name: "Formatted multiple keys",
			input: `{
						"keyA": {
							"inner" : "value"
						},
						"keyB": "V2"        
					}`,
			wants: "keyA",
		},
		{
			name:       "Invalid input",
			input:      `non json array`,
			wantsError: "invalid JSON payload, failed to find the opening bracket",
		},
		{
			name:       "Empty JSON input",
			input:      `{}`,
			wantsError: "invalid JSON payload, expected a key but found none",
		},
		{
			name:       "Broken JSON input",
			input:      `{ "key}`,
			wantsError: "invalid JSON payload",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, err := extractFirstKey([]byte(test.input))

			if test.wantsError != "" {
				require.Error(t, err)
				require.Equal(t, test.wantsError, err.Error())
			}

			require.Equal(t, test.wants, key)
		})
	}
}

func TestDetectTriggerType(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		want         eventType
		isError      bool
		errorContent string
	}{
		{
			name:  "Parse S3 event",
			input: []byte(`{"Records": "some content"}`),
			want:  s3Event,
		},
		{
			name:         "Invalid event - unknown json",
			input:        []byte(`{"key": "value"}`),
			isError:      true,
			errorContent: "unknown event type",
		},
		{
			name:         "Invalid event - no JSON key",
			input:        []byte(`invalid content`),
			isError:      true,
			errorContent: "invalid JSON payload",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detectTriggerType(tt.input)
			if tt.errorContent != "" {
				require.ErrorContains(t, err, tt.errorContent)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHandleCustomTrigger(t *testing.T) {
	commonCfg := Config{FailureBucketARN: "aws:s3:::my-bucket"}

	commonLogger := zap.NewNop()
	goMock := gomock.NewController(t)

	s3Provider := internal.NewMockS3Provider(goMock)

	tests := []struct {
		name              string
		handlerFunc       func() internal.CustomTriggerHandler
		expectHandleCount int
		expectError       string
	}{
		{
			name: "Successful event handling - two events",
			handlerFunc: func() internal.CustomTriggerHandler {
				handler := internal.NewMockCustomTriggerHandler(goMock)
				handler.EXPECT().HasNext(t.Context()).Times(2).Return(true)
				handler.EXPECT().HasNext(t.Context()).Times(1).Return(false)
				handler.EXPECT().PostProcess(gomock.Any()).AnyTimes()
				handler.EXPECT().IsDryRun().AnyTimes().Return(false)
				handler.EXPECT().Error().AnyTimes().Return(nil)

				handler.EXPECT().
					GetNext(gomock.Any()).Times(2).
					Return([]byte(`{"Records": "S3 event content"}`), nil)

				return handler
			},
			expectHandleCount: 2,
		},
		{
			name: "Event handling is skipped for dry-run mode",
			handlerFunc: func() internal.CustomTriggerHandler {
				handler := internal.NewMockCustomTriggerHandler(goMock)
				handler.EXPECT().HasNext(t.Context()).Times(1).Return(true)
				handler.EXPECT().HasNext(t.Context()).Times(1).Return(false)
				handler.EXPECT().PostProcess(gomock.Any()).AnyTimes()
				handler.EXPECT().Error().AnyTimes().Return(nil)

				// set dry-run mode to true
				handler.EXPECT().IsDryRun().AnyTimes().Return(true)

				handler.EXPECT().
					GetNext(gomock.Any()).Times(1).
					Return([]byte(`{"Records": "S3 event content"}`), nil)

				return handler
			},
			expectHandleCount: 0,
		},
		{
			name: "Event handling fails for unknown event type",
			handlerFunc: func() internal.CustomTriggerHandler {
				handler := internal.NewMockCustomTriggerHandler(goMock)
				handler.EXPECT().HasNext(t.Context()).Times(1).Return(true)
				handler.EXPECT().PostProcess(gomock.Any()).AnyTimes()
				handler.EXPECT().IsDryRun().AnyTimes().Return(false)
				handler.EXPECT().Error().AnyTimes().Return(nil)

				handler.EXPECT().
					GetNext(gomock.Any()).Times(1).
					Return([]byte(`{"test": "unknown trigger"}`), nil)

				return handler
			},
			expectHandleCount: 0,
			expectError:       "unknown event type with key: test",
		},
		{
			name: "Event handling ends with error when handler returns error",
			handlerFunc: func() internal.CustomTriggerHandler {
				handler := internal.NewMockCustomTriggerHandler(goMock)
				handler.EXPECT().HasNext(t.Context()).Times(1).Return(false)

				// set error on the handler
				handler.EXPECT().Error().Return(errors.New("some error from handler"))

				return handler
			},
			expectHandleCount: 0,
			expectError:       "some error from handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.handlerFunc()

			mockHandler := mockPlogEventHandler{event: s3Event}
			hp := mockHandlerProvider{&mockHandler}

			receiver := awsLambdaReceiver{
				cfg:        &commonCfg,
				logger:     commonLogger,
				hp:         &hp,
				s3Provider: s3Provider,
			}

			err := receiver.handleCustomTriggers(t.Context(), handler)

			if tt.expectError != "" {
				require.ErrorContains(t, err, tt.expectError)
			}

			require.Equal(t, tt.expectHandleCount, mockHandler.handleCount)
		})
	}
}

type mockExtensionWithPLogUnmarshaler struct {
	mockExtension    // Embed the base mock implementation.
	plog.Unmarshaler // Add the unmarshaler interface when needed.
}

type mockExtensionWithPMetricUnmarshaler struct {
	mockExtension       // Embed the base mock implementation.
	pmetric.Unmarshaler // Add the unmarshaler interface when needed.
}

type mockExtension struct{}

func (*mockExtension) Start(_ context.Context, _ component.Host) error {
	// Mock the behavior of the Start method.
	return nil
}

func (*mockExtension) Shutdown(_ context.Context) error {
	// Mock the behavior of the Shutdown method.
	return nil
}

type mockHost struct {
	GetFunc func() map[component.ID]component.Component
	_       struct{} // prevent unkeyed literal initialization
}

func (m mockHost) GetExtensions() map[component.ID]component.Component {
	return m.GetFunc()
}

type unmarshalLogsFunc func([]byte) (plog.Logs, error)

func (f unmarshalLogsFunc) UnmarshalLogs(data []byte) (plog.Logs, error) {
	return f(data)
}

type unmarshalMetricsFunc func([]byte) (pmetric.Metrics, error)

func (f unmarshalMetricsFunc) UnmarshalMetrics(data []byte) (pmetric.Metrics, error) {
	return f(data)
}
