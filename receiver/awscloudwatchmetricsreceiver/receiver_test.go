// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

const (
	namespace  = "AWS/EC2"
	metricname = "CPUUtilization"
	agg        = "Average"
	DimName    = "InstanceId"
	DimValue   = "i-1234567890abcdef0"
)

func TestDefaultFactory(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "eu-west-1"

	sink := &consumertest.MetricsSink{}
	mtrcRcvr := newMetricReceiver(cfg, zap.NewNop(), sink)

	err := mtrcRcvr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = mtrcRcvr.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestGroupConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "eu-west-1"
	cfg.PollInterval = time.Minute * 5
	cfg.Metrics = &MetricsConfig{
		Group: []GroupConfig{
			{
				Namespace: namespace,
				Period:    time.Second * 60 * 5,
				MetricName: []NamedConfig{
					{
						MetricName:     metricname,
						AwsAggregation: agg,
						Dimensions: []MetricDimensionsConfig{
							{
								Name:  DimName,
								Value: DimValue,
							},
						},
					},
				},
			},
		},
	}
	sink := &consumertest.MetricsSink{}
	mtrcRcvr := newMetricReceiver(cfg, zap.NewNop(), sink)
	mtrcRcvr.client = defaultMockClient()

	err := mtrcRcvr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.DataPointCount() > 0
	}, 2*time.Second, 10*time.Millisecond)

	groupRequests := mtrcRcvr.groupRequests
	require.Len(t, groupRequests, 1)
	require.Equal(t, groupRequests[0].groupName(), "test-log-group-name")

	err = mtrcRcvr.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]
	expected, err := golden.ReadLogs(filepath.Join("testdata", "processed", "prefixed.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expected, logs, plogtest.IgnoreObservedTimestamp()))
}

func defaultMockClient() client {
	mc := &mockClient{}
	mc.On()
}

type mockClient struct {
	mock.Mock
}
