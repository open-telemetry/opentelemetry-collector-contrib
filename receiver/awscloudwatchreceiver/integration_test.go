// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

func TestLoggingIntegration(t *testing.T) {
	mc := &mockClient{}
	mc.On("DescribeLogGroups", mock.Anything, mock.Anything, mock.Anything).
		Return(loadLogGroups(t), nil)

	mc.On("FilterLogEvents", mock.Anything, mock.Anything, mock.Anything).
		Return(loadLogEvents(t), nil)

	sink := &consumertest.LogsSink{}
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-east-2"
	cfg.Logs.PollInterval = time.Second
	cfg.Logs.Groups.AutodiscoverConfig = &AutodiscoverConfig{
		Limit: 1,
	}
	recv, err := NewFactory().CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		sink,
	)
	require.NoError(t, err)

	rcvr, ok := recv.(*logsReceiver)
	require.True(t, ok)
	rcvr.client = mc

	err = recv.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 5*time.Second, 10*time.Millisecond)

	err = recv.Shutdown(t.Context())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]

	expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "golden", "autodiscovered.yaml"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreObservedTimestamp()))
}

// TestMetricsIntegration_Summary verifies that the metrics scraper produces the expected
// pdata structure in summary mode (no explicit Stats → Summary metric type) against a
// golden file so that format changes are caught on subsequent PRs.
func TestMetricsIntegration_Summary(t *testing.T) {
	mc := &mockMetricsClient{}
	ts := time.Unix(1_700_000_000, 0).UTC()

	mc.On("GetMetricData", mock.Anything, mock.Anything, mock.Anything).
		Return(&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []cwtypes.MetricDataResult{
				{Id: aws.String("q0_0"), Values: []float64{80.0}, Timestamps: []time.Time{ts}}, // Sum
				{Id: aws.String("q0_1"), Values: []float64{4.0}, Timestamps: []time.Time{ts}},  // SampleCount
				{Id: aws.String("q0_2"), Values: []float64{10.0}, Timestamps: []time.Time{ts}}, // Minimum
				{Id: aws.String("q0_3"), Values: []float64{30.0}, Timestamps: []time.Time{ts}}, // Maximum
			},
		}, nil)

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-east-1"
	cfg.Metrics.Queries = []MetricQuery{
		{
			Namespace:  "AWS/EC2",
			MetricName: "CPUUtilization",
			Dimensions: map[string]string{"InstanceId": "i-1234567890abcdef0"},
		},
	}

	scr := newCloudWatchMetricsScraper(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	})
	scr.client = mc

	md, err := scr.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())

	goldenPath := filepath.Join("testdata", "golden", "metrics-summary.yaml")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, golden.WriteMetricsToFile(goldenPath, md))
	}
	expected, err := golden.ReadMetrics(goldenPath)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, md,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
	))
}

// TestMetricsIntegration_Gauge verifies the gauge mode output (explicit Stats list →
// one Gauge data point per stat, tagged with a "stat" attribute) against a golden file.
func TestMetricsIntegration_Gauge(t *testing.T) {
	mc := &mockMetricsClient{}
	ts := time.Unix(1_700_000_000, 0).UTC()

	mc.On("GetMetricData", mock.Anything, mock.Anything, mock.Anything).
		Return(&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []cwtypes.MetricDataResult{
				{Id: aws.String("q0_0"), Values: []float64{85.5}, Timestamps: []time.Time{ts}}, // Average
				{Id: aws.String("q0_1"), Values: []float64{95.2}, Timestamps: []time.Time{ts}}, // p99
			},
		}, nil)

	cfg := createDefaultConfig().(*Config)
	cfg.Region = "us-east-1"
	cfg.Metrics.Queries = []MetricQuery{
		{
			Namespace:  "AWS/EC2",
			MetricName: "CPUUtilization",
			Dimensions: map[string]string{"InstanceId": "i-1234567890abcdef0"},
			Stats:      []string{"Average", "p99"},
		},
	}

	scr := newCloudWatchMetricsScraper(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	})
	scr.client = mc

	md, err := scr.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())

	goldenPath := filepath.Join("testdata", "golden", "metrics-gauge.yaml")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, golden.WriteMetricsToFile(goldenPath, md))
	}
	expected, err := golden.ReadMetrics(goldenPath)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, md,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
	))
}

var (
	logGroupFiles = []string{
		filepath.Join("testdata", "log-groups", "group-1.json"),
	}
	logEventsFiles = []string{
		filepath.Join("testdata", "events", "event-1.json"),
	}
)

func loadLogGroups(t *testing.T) *cloudwatchlogs.DescribeLogGroupsOutput {
	output := make([]types.LogGroup, len(logGroupFiles))
	for i, lg := range logGroupFiles {
		bytes, err := os.ReadFile(lg)
		require.NoError(t, err)
		var logGroup types.LogGroup
		err = json.Unmarshal(bytes, &logGroup)
		require.NoError(t, err)
		output[i] = logGroup
	}

	return &cloudwatchlogs.DescribeLogGroupsOutput{
		LogGroups: output,
		NextToken: nil,
	}
}

func loadLogEvents(t *testing.T) *cloudwatchlogs.FilterLogEventsOutput {
	output := make([]types.FilteredLogEvent, len(logEventsFiles))
	for i, lg := range logEventsFiles {
		bytes, err := os.ReadFile(lg)
		require.NoError(t, err)
		var event types.FilteredLogEvent
		err = json.Unmarshal(bytes, &event)
		require.NoError(t, err)
		output[i] = event
	}

	return &cloudwatchlogs.FilterLogEventsOutput{
		Events:    output,
		NextToken: nil,
	}
}
