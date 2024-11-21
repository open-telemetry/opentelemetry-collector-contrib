// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal/mocks"
)

func stringPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}

func TestNewReceiver(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Second,
		},
	}
	mr := newHuaweiCloudCesReceiver(receivertest.NewNopSettings(), cfg, new(consumertest.MetricsSink))
	assert.NotNil(t, mr)
}

func TestListMetricDefinitionsSuccess(t *testing.T) {
	mockCes := mocks.NewCesClient(t)

	mockResponse := &model.ListMetricsResponse{
		Metrics: &[]model.MetricInfoList{
			{
				Namespace:  "SYS.ECS",
				MetricName: "cpu_util",
				Dimensions: []model.MetricsDimension{
					{
						Name:  "instance_id",
						Value: "12345",
					},
				},
			},
		},
	}

	mockCes.On("ListMetrics", mock.Anything).Return(mockResponse, nil)

	receiver := &cesReceiver{
		client: mockCes,
		config: createDefaultConfig().(*Config),
	}

	metrics, err := receiver.listMetricDefinitions(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Equal(t, "SYS.ECS", metrics[0].Namespace)
	assert.Equal(t, "cpu_util", metrics[0].MetricName)
	assert.Equal(t, "instance_id", metrics[0].Dimensions[0].Name)
	assert.Equal(t, "12345", metrics[0].Dimensions[0].Value)
	mockCes.AssertExpectations(t)
}

func TestListMetricDefinitionsFailure(t *testing.T) {
	mockCes := mocks.NewCesClient(t)

	mockCes.On("ListMetrics", mock.Anything).Return(nil, errors.New("failed to list metrics"))
	receiver := &cesReceiver{
		client: mockCes,
		config: createDefaultConfig().(*Config),
	}

	metrics, err := receiver.listMetricDefinitions(context.Background())

	assert.Error(t, err)
	assert.Empty(t, metrics)
	assert.Equal(t, "failed to list metrics", err.Error())
	mockCes.AssertExpectations(t)
}

func TestListDataPointsForMetricBackOffWIthDefaultConfig(t *testing.T) {
	mockCes := mocks.NewCesClient(t)
	next := new(consumertest.MetricsSink)
	receiver := newHuaweiCloudCesReceiver(receivertest.NewNopSettings(), createDefaultConfig().(*Config), next)
	receiver.client = mockCes

	mockCes.On("ShowMetricData", mock.Anything).Return(nil, errors.New(requestThrottledErrMsg)).Times(3)
	mockCes.On("ShowMetricData", mock.Anything).Return(&model.ShowMetricDataResponse{
		MetricName: stringPtr("cpu_util"),
		Datapoints: &[]model.Datapoint{
			{
				Average:   float64Ptr(45.67),
				Timestamp: 1556625610000,
			},
			{
				Average:   float64Ptr(89.01),
				Timestamp: 1556625715000,
			},
		},
	}, nil)

	resp, err := receiver.listDataPointsForMetric(context.Background(), time.Now().Add(10*time.Minute), time.Now(), model.MetricInfoList{
		Namespace:  "SYS.ECS",
		MetricName: "cpu_util",
		Dimensions: []model.MetricsDimension{
			{
				Name:  "instance_id",
				Value: "12345",
			},
		},
	})

	require.NoError(t, err)
	assert.Len(t, *resp.Datapoints, 2)
}

func TestListDataPointsForMetricBackOffFails(t *testing.T) {
	mockCes := mocks.NewCesClient(t)
	next := new(consumertest.MetricsSink)
	receiver := newHuaweiCloudCesReceiver(receivertest.NewNopSettings(), &Config{BackOffConfig: configretry.BackOffConfig{
		Enabled:             true,
		InitialInterval:     100 * time.Millisecond,
		MaxInterval:         800 * time.Millisecond,
		MaxElapsedTime:      1 * time.Second,
		RandomizationFactor: 0,
		Multiplier:          2,
	}}, next)
	receiver.client = mockCes

	mockCes.On("ShowMetricData", mock.Anything).Return(nil, errors.New(requestThrottledErrMsg)).Times(4)

	resp, err := receiver.listDataPointsForMetric(context.Background(), time.Now().Add(10*time.Minute), time.Now(), model.MetricInfoList{
		Namespace:  "SYS.ECS",
		MetricName: "cpu_util",
		Dimensions: []model.MetricsDimension{
			{
				Name:  "instance_id",
				Value: "12345",
			},
		},
	})

	require.ErrorContains(t, err, requestThrottledErrMsg)
	assert.Nil(t, resp)
}

func TestPollMetricsAndConsumeSuccess(t *testing.T) {
	mockCes := mocks.NewCesClient(t)
	next := new(consumertest.MetricsSink)
	receiver := newHuaweiCloudCesReceiver(receivertest.NewNopSettings(), &Config{}, next)
	receiver.client = mockCes

	mockCes.On("ListMetrics", mock.Anything).Return(&model.ListMetricsResponse{
		Metrics: &[]model.MetricInfoList{
			{
				Namespace:  "SYS.ECS",
				MetricName: "cpu_util",
				Dimensions: []model.MetricsDimension{
					{
						Name:  "instance_id",
						Value: "12345",
					},
				},
			},
		},
	}, nil)

	mockCes.On("ShowMetricData", mock.Anything).Return(&model.ShowMetricDataResponse{
		MetricName: stringPtr("cpu_util"),
		Datapoints: &[]model.Datapoint{
			{
				Average:   float64Ptr(45.67),
				Timestamp: 1556625610000,
			},
			{
				Average:   float64Ptr(89.01),
				Timestamp: 1556625715000,
			},
		},
	}, nil)

	err := receiver.pollMetricsAndConsume(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, next.DataPointCount())
}

func TestStartReadingMetrics(t *testing.T) {
	tests := []struct {
		name                    string
		scrapeInterval          time.Duration
		setupMocks              func(*mocks.CesClient)
		expectedNumOfDataPoints int
	}{
		{
			name:           "Success case with valid scrape interval",
			scrapeInterval: 2 * time.Second,
			setupMocks: func(m *mocks.CesClient) {
				m.On("ListMetrics", mock.Anything).Return(&model.ListMetricsResponse{
					Metrics: &[]model.MetricInfoList{
						{
							Namespace:  "SYS.ECS",
							MetricName: "cpu_util",
							Dimensions: []model.MetricsDimension{
								{
									Name:  "instance_id",
									Value: "12345",
								},
							},
						},
					},
				}, nil)

				m.On("ShowMetricData", mock.Anything).Return(&model.ShowMetricDataResponse{
					MetricName: stringPtr("cpu_util"),
					Datapoints: &[]model.Datapoint{
						{
							Average:   float64Ptr(45.67),
							Timestamp: 1556625610000,
						},
					},
				}, nil)
			},
			expectedNumOfDataPoints: 1,
		},
		{
			name:           "Error case with Scrape returning error",
			scrapeInterval: 1 * time.Second,
			setupMocks: func(m *mocks.CesClient) {
				m.On("ListMetrics", mock.Anything).Return(nil, errors.New("server error"))
			},
			expectedNumOfDataPoints: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCes := mocks.NewCesClient(t)
			next := new(consumertest.MetricsSink)
			tt.setupMocks(mockCes)
			logger := zaptest.NewLogger(t)
			r := &cesReceiver{
				config: &Config{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: tt.scrapeInterval,
						InitialDelay:       10 * time.Millisecond,
					},
				},
				client:       mockCes,
				logger:       logger,
				nextConsumer: next,
				lastSeenTs:   make(map[string]time.Time),
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r.startReadingMetrics(ctx)

			assert.Equal(t, tt.expectedNumOfDataPoints, next.DataPointCount())
		})
	}
}
