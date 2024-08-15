package huaweicloudcesreceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/mocks"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
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
	}

	metrics, err := receiver.listMetricDefinitions()

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
	}

	metrics, err := receiver.listMetricDefinitions()

	assert.Error(t, err)
	assert.Len(t, metrics, 0)
	assert.Equal(t, "failed to list metrics", err.Error())
	mockCes.AssertExpectations(t)
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

	mockCes.On("BatchListMetricData", mock.Anything).Return(&model.BatchListMetricDataResponse{
		Metrics: &[]model.BatchMetricData{
			{
				Namespace:  stringPtr("SYS.ECS"),
				MetricName: "cpu_util",
				Dimensions: &[]model.MetricsDimension{
					{
						Name:  "instance_id",
						Value: "faea5b75-e390-4e2b-8733-9226a9026070",
					},
				},
				Datapoints: []model.DatapointForBatchMetric{
					{
						Average:   float64Ptr(45.67),
						Timestamp: 1556625610000,
					},
					{
						Average:   float64Ptr(89.01),
						Timestamp: 1556625715000,
					},
				},
				Unit: stringPtr("%"),
			},
		},
	}, nil)

	err := receiver.pollMetricsAndConsume(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, next.DataPointCount())
}
