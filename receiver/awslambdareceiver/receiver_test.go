// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awslambdareceiver

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
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
	// Create receiver using factory with default config. This means decoding
	// is done using the default CloudWatch Logs subscription filter unmarshaler.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sink := consumertest.LogsSink{}
	receiver, err := factory.CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		&sink,
	)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	// Initialize the handler manually (without starting Lambda runtime)
	awsReceiver := receiver.(*awsLambdaReceiver)
	handler, err := awsReceiver.newHandler(t.Context(), componenttest.NewNopHost(), nil)
	require.NoError(t, err)
	awsReceiver.handler = handler

	// Create a fake CloudWatch lambda event using testdata
	// Read and prepare the CloudWatch log data
	testDataPath := filepath.Join("testdata", "cloudwatch_log.json")
	testData := getDataFromFile(t, testDataPath)

	// Process a CloudWatch logs event.
	lambdaEvent, err := json.Marshal(events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: testData,
		},
	})
	require.NoError(t, err)
	err = awsReceiver.processLambdaEvent(t.Context(), lambdaEvent)
	require.NoError(t, err)

	// Verify logs were sent to the sink
	require.NotZero(t, sink.LogRecordCount(), "Expected logs to be sent to sink")
}

func TestCreateMetrics(t *testing.T) {
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

	host := MockHost{GetFunc: func() map[component.ID]component.Component {
		return map[component.ID]component.Component{
			component.MustNewID("dummy_metric_encoding"): &MockExtensionWithPMetricUnmarshaler{
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

	// Initialize the handler manually (without starting Lambda runtime).
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

func TestProcessLambdaEvent(t *testing.T) {
	commonCfg := Config{
		S3Encoding:       "alb",
		FailureBucketARN: "aws:s3:::my-bucket",
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
			name:      "Successful CloudWatch event",
			eventType: cwEvent,
			event:     []byte(`{"awslogs": "CW event content"}`),
		},
		{
			name:         "Wrong event type - S3 event for CW handler",
			eventType:    cwEvent,
			event:        []byte(`{"Records": "S3 event content"}`),
			isError:      true,
			errorContent: `cannot handle event type`,
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

func TestHandleCustomTrigger(t *testing.T) {
	commonCfg := Config{
		S3Encoding:       "alb",
		FailureBucketARN: "aws:s3:::my-bucket",
	}

	commonLogger := zap.NewNop()
	goMock := gomock.NewController(t)

	s3Provider := internal.NewMockS3Provider(goMock)

	t.Run("Successful event handling for S3 event", func(t *testing.T) {
		customEventHandler := internal.NewMockCustomEventHandler[*internal.ErrorReplayResponse](goMock)
		customEventHandler.EXPECT().IsDryRun().AnyTimes().Return(false)
		customEventHandler.EXPECT().HasNext().Times(1).Return(true)
		customEventHandler.EXPECT().HasNext().Times(1).Return(false)
		customEventHandler.EXPECT().PostProcess(gomock.Any(), gomock.Any()).AnyTimes()

		customEventHandler.EXPECT().GetNext(gomock.Any()).Times(1).Return(
			&internal.ErrorReplayResponse{
				Content: []byte(`{"Records": "S3 event content"}`),
				Key:     "my-object-key",
			}, nil)

		mockHandler := mockPlogEventHandler{event: s3Event}

		receiver := awsLambdaReceiver{
			cfg:        &commonCfg,
			logger:     commonLogger,
			handler:    &mockHandler,
			s3Provider: s3Provider,
		}

		err := receiver.handleCustomTriggers(t.Context(), customEventHandler)
		require.NoError(t, err)
		require.Equal(t, 1, mockHandler.handleCount)
	})

	t.Run("Dry-run mode skips event processing", func(t *testing.T) {
		customEventHandler := internal.NewMockCustomEventHandler[*internal.ErrorReplayResponse](goMock)
		customEventHandler.EXPECT().HasNext().Times(1).Return(true)
		customEventHandler.EXPECT().HasNext().Times(1).Return(false)
		customEventHandler.EXPECT().PostProcess(gomock.Any(), gomock.Any()).AnyTimes()

		// set dry-run mode
		customEventHandler.EXPECT().IsDryRun().AnyTimes().Return(true)

		customEventHandler.EXPECT().GetNext(gomock.Any()).Times(1).Return(
			&internal.ErrorReplayResponse{
				Content: []byte(`{"Records": "S3 event content"}`),
				Key:     "my-object-key",
			}, nil)

		mockHandler := mockPlogEventHandler{event: s3Event}
		mockConsumer := noOpLogsConsumer{}

		receiver := awsLambdaReceiver{
			cfg:        &commonCfg,
			logger:     commonLogger,
			handler:    &mockHandler,
			s3Provider: s3Provider,
		}

		err := receiver.handleCustomTriggers(t.Context(), customEventHandler)
		require.NoError(t, err)
		require.Equal(t, 0, mockHandler.handleCount)
		require.Equal(t, 0, mockConsumer.consumeCount)
	})
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
			name: "Default to CW Subscription filter - success",
			factoryMock: func(_ context.Context, _ extension.Settings, _ component.Config) (extension.Extension, error) {
				return &MockExtensionWithPLogUnmarshaler{}, nil
			},
			isErr:               false,
			expectedHandlerType: reflect.TypeOf(&cwLogsSubscriptionHandler{}),
		},
		{
			name:       "Prioritize ID based loading - success",
			s3Encoding: "my_encoding",
			hostMock: func() map[component.ID]component.Component {
				id := component.NewID(component.MustNewType("my_encoding"))

				return map[component.ID]component.Component{
					id: &MockExtensionWithPLogUnmarshaler{},
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
			host := MockHost{GetFunc: tt.hostMock}
			factory := MockExtFactory{CreateFunc: tt.factoryMock}

			// load handler and validate
			handler, err := newLogsHandler(
				t.Context(),
				&Config{S3Encoding: tt.s3Encoding},
				receivertest.NewNopSettings(metadata.Type),
				host,
				consumertest.NewNop(),
				s3Provider,
				&factory,
			)
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
					id: &MockExtensionWithPLogUnmarshaler{},
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
					id: &MockExtensionWithPLogUnmarshaler{},
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
					id: &MockExtensionWithPMetricUnmarshaler{},
				}
			},
			isError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := MockHost{
				tt.hostGetMock,
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

func TestLoadSubFilterLogUnmarshaler(t *testing.T) {
	tests := []struct {
		name        string
		mockFactory func(ctx context.Context, settings extension.Settings, config component.Config) (extension.Extension, error)
		expectedErr string
	}{
		{
			name: "successful_case",
			mockFactory: func(_ context.Context, _ extension.Settings, _ component.Config) (extension.Extension, error) {
				return &MockExtensionWithPLogUnmarshaler{}, nil
			},
		},
		{
			name: "create_extension_error",
			mockFactory: func(_ context.Context, _ extension.Settings, _ component.Config) (extension.Extension, error) {
				return nil, errors.New("mock factory creation error")
			},
			expectedErr: "failed to create extension",
		},
		{
			name: "invalid_unmarshaler",
			mockFactory: func(_ context.Context, _ extension.Settings, _ component.Config) (extension.Extension, error) {
				return &MockExtension{}, nil
			},
			expectedErr: "does not implement plog.Unmarshaler",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockFactory := &MockExtFactory{
				CreateFunc: test.mockFactory,
			}

			_, errP := loadSubFilterLogUnmarshaler(t.Context(), mockFactory)
			if test.expectedErr != "" {
				require.ErrorContains(t, errP, test.expectedErr)
			} else {
				require.NoError(t, errP)
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
			name:  "Parse CloudWatch event",
			input: []byte(`{"awslogs": "some content"}`),
			want:  cwEvent,
		},
		{
			name:  "Custom event handling",
			input: []byte(`{"replayFailedEvents": "some content"}`),
			want:  customReplayEvent,
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
			if err != nil {
				require.ErrorContains(t, err, tt.errorContent)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

type MockExtFactory struct {
	CreateFunc func(ctx context.Context, settings extension.Settings, config component.Config) (extension.Extension, error)
}

func (m *MockExtFactory) Create(ctx context.Context, settings extension.Settings, config component.Config) (extension.Extension, error) {
	return m.CreateFunc(ctx, settings, config)
}

type MockExtensionWithPLogUnmarshaler struct {
	MockExtension    // Embed the base mock implementation.
	plog.Unmarshaler // Add the unmarshaler interface when needed.
}

type MockExtensionWithPMetricUnmarshaler struct {
	MockExtension       // Embed the base mock implementation.
	pmetric.Unmarshaler // Add the unmarshaler interface when needed.
}

type MockExtension struct{}

func (m *MockExtension) Start(_ context.Context, _ component.Host) error {
	// Mock the behavior of the Start method.
	return nil
}

func (m *MockExtension) Shutdown(_ context.Context) error {
	// Mock the behavior of the Shutdown method.
	return nil
}

type MockHost struct {
	GetFunc func() map[component.ID]component.Component
}

func (m MockHost) GetExtensions() map[component.ID]component.Component {
	return m.GetFunc()
}

type unmarshalMetricsFunc func([]byte) (pmetric.Metrics, error)

func (f unmarshalMetricsFunc) UnmarshalMetrics(data []byte) (pmetric.Metrics, error) {
	return f(data)
}
