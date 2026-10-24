// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package huaweicloudcesreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal/mocks"
)

func TestHuaweiCloudCESReceiverIntegration(t *testing.T) {
	mc := mocks.NewCesClient(t)

	mc.On("ListMetrics", mock.Anything).Return(&model.ListMetricsResponse{
		Metrics: &[]model.MetricInfoList{
			{
				Namespace:  "SYS.ECS",
				MetricName: "cpu_util",
				Dimensions: []model.MetricsDimensionResp{
					{
						Name:  new("instance_id"),
						Value: new("faea5b75-e390-4e2b-8733-9226a9026070"),
					},
				},
				Unit: "%",
			},
			{
				Namespace:  "SYS.ECS",
				MetricName: "mem_util",
				Dimensions: []model.MetricsDimensionResp{
					{
						Name:  new("instance_id"),
						Value: new("abcea5b75-e390-4e2b-8733-9226a9026070"),
					},
				},
				Unit: "%",
			},
			{
				Namespace:  "SYS.VPC",
				MetricName: "upstream_bandwidth_usage",
				Dimensions: []model.MetricsDimensionResp{
					{
						Name:  new("publicip_id"),
						Value: new("faea5b75-e390-4e2b-8733-9226a9026070"),
					},
				},
				Unit: "%",
			},
		},
	}, nil)

	mc.On("ShowMetricData", mock.Anything).Return(&model.ShowMetricDataResponse{
		MetricName: new("cpu_util"),
		Datapoints: &[]model.Datapoint{
			{
				Average:   new(float64(10)),
				Timestamp: 1556625610000,
			},
			{
				Average:   new(float64(20)),
				Timestamp: 1556625715000,
			},
		},
	}, nil).Times(1)
	mc.On("ShowMetricData", mock.Anything).Return(&model.ShowMetricDataResponse{
		MetricName: new("mem_util"),
		Datapoints: &[]model.Datapoint{
			{
				Average:   new(float64(30)),
				Timestamp: 1556625610000,
			},
			{
				Average:   new(float64(40)),
				Timestamp: 1556625715000,
			},
		},
	}, nil).Times(1)
	mc.On("ShowMetricData", mock.Anything).Return(&model.ShowMetricDataResponse{
		MetricName: new("upstream_bandwidth_usage"),
		Datapoints: &[]model.Datapoint{
			{
				Average:   new(float64(50)),
				Timestamp: 1556625610000,
			},
			{
				Average:   new(float64(60)),
				Timestamp: 1556625715000,
			},
		},
	}, nil).Times(1)

	sink := &consumertest.MetricsSink{}
	cfg := createDefaultConfig().(*Config)
	cfg.RegionID = "us-east-2"
	cfg.ControllerConfig.CollectionInterval = time.Second
	cfg.ProjectID = "my-project"
	cfg.Filter = "average"

	recv, err := NewFactory().CreateMetrics(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		sink,
	)
	require.NoError(t, err)

	rcvr, ok := recv.(*cesReceiver)
	require.True(t, ok)
	rcvr.client = mc

	err = recv.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.DataPointCount() > 0
	}, 5*time.Second, 10*time.Millisecond)

	err = recv.Shutdown(t.Context())
	require.NoError(t, err)

	metrics := sink.AllMetrics()[0]

	expectedMetrics, err := golden.ReadMetrics(filepath.Join("testdata", "golden", "metrics_golden.yaml"))
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, metrics, pmetrictest.IgnoreResourceMetricsOrder()))
}
