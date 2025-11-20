// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awslambdareceiver

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension"
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
	t.Setenv("_LAMBDA_SERVER_PORT", "9001")
	t.Setenv("AWS_LAMBDA_RUNTIME_API", "localhost:9001")

	// Create receiver using factory with S3 encoding config.
	// Note: The S3Encoding value must match the component ID used when registering the extension.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.S3Encoding = awsLogsEncoding
	sink := consumertest.LogsSink{}
	receiver, err := factory.CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
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
			component.MustNewID(awsLogsEncoding): &mockExtensionWithPLogUnmarshaler{
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

	// Initialize the handler manually (without starting Lambda runtime to avoid goroutine leaks in tests)
	awsReceiver := receiver.(*awsLambdaReceiver)
	handler, err := awsReceiver.newHandler(t.Context(), host, s3Provider)
	require.NoError(t, err)
	awsReceiver.handler = handler

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
	t.Setenv("_LAMBDA_SERVER_PORT", "9001")
	t.Setenv("AWS_LAMBDA_RUNTIME_API", "localhost:9001")

	// Create receiver using factory with a dummy encoding extension.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.S3Encoding = "dummy_metric_encoding"
	sink := consumertest.MetricsSink{}
	receiver, err := factory.CreateMetrics(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
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
			component.MustNewID("dummy_metric_encoding"): &mockExtensionWithPMetricUnmarshaler{
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

	// Initialize the handler manually (without starting Lambda runtime to avoid goroutine leaks in tests)
	awsReceiver := receiver.(*awsLambdaReceiver)
	handler, err := awsReceiver.newHandler(t.Context(), host, s3Provider)
	require.NoError(t, err)
	awsReceiver.handler = handler

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

func TestCreateMetricsRequiresS3Encoding(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetrics(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	require.EqualError(t, err, "s3_encoding is required for metrics")
}

func TestStartRequiresLambdaEnvironment(t *testing.T) {
	// Ensure Lambda environment variables are not set
	t.Setenv("_LAMBDA_SERVER_PORT", "")
	t.Setenv("AWS_LAMBDA_RUNTIME_API", "")

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.S3Encoding = "test_encoding"

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
		S3Encoding: "alb",
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
			mockHandler := mockPlogEventHandler{event: tt.eventType}
			receiver := awsLambdaReceiver{
				cfg:     &commonCfg,
				logger:  commonLogger,
				handler: &mockHandler,
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

func TestLoadLogsHandler(t *testing.T) {
	tests := []struct {
		name                string
		s3Encoding          string
		factoryMock         func(ctx context.Context, settings extension.Settings, config component.Config) (extension.Extension, error)
		hostMock            func() map[component.ID]component.Component
		expectedHandlerType reflect.Type
		isErr               bool
	}{
		{
			name:       "Prioritize ID based loading - success",
			s3Encoding: "my_encoding",
			hostMock: func() map[component.ID]component.Component {
				id := component.NewID(component.MustNewType("my_encoding"))

				return map[component.ID]component.Component{
					id: &mockExtensionWithPLogUnmarshaler{},
				}
			},
			isErr:               false,
			expectedHandlerType: reflect.TypeOf(&s3Handler[plog.Logs]{}),
		},
		{
			name:       "Error if handler loading fails",
			s3Encoding: "custom_encoding",
			hostMock: func() map[component.ID]component.Component {
				// no registered extensions matching s3Encoding
				return map[component.ID]component.Component{}
			},
			isErr: true,
		},
	}

	ctr := gomock.NewController(t)
	s3Service := internal.NewMockS3Service(ctr)
	s3Provider := internal.NewMockS3Provider(ctr)
	s3Provider.EXPECT().GetService(gomock.Any()).AnyTimes().Return(s3Service, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// construct mocks
			host := mockHost{GetFunc: tt.hostMock}

			// load handler and validate
			handler, err := newLogsHandler(
				t.Context(),
				&Config{S3Encoding: tt.s3Encoding},
				receivertest.NewNopSettings(metadata.Type),
				host,
				consumertest.NewNop(),
				s3Provider)
			if tt.isErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// validate handler type
				if reflect.TypeOf(handler) != tt.expectedHandlerType {
					t.Errorf("expected handler to be of type %v; got %v", tt.expectedHandlerType, reflect.TypeOf(handler))
				}
			}
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
