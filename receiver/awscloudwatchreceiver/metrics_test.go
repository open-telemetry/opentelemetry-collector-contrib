// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

// mockMetricsClient implements metricsClient for testing.
type mockMetricsClient struct {
	mock.Mock
}

func (m *mockMetricsClient) ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*cloudwatch.ListMetricsOutput), args.Error(1)
}

func (m *mockMetricsClient) GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*cloudwatch.GetMetricDataOutput), args.Error(1)
}

func testScraper(cfg *Config) *cloudWatchMetricsScraper {
	defaults := createDefaultConfig().(*Config).Metrics
	if cfg.Metrics.Period == 0 {
		cfg.Metrics.Period = defaults.Period
	}
	if cfg.Metrics.Delay == 0 {
		cfg.Metrics.Delay = defaults.Delay
	}
	if cfg.Metrics.CollectionInterval == 0 {
		cfg.Metrics.CollectionInterval = defaults.CollectionInterval
	}
	return newCloudWatchMetricsScraper(cfg, receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	})
}

// --- pure-function tests ---

func TestAlignTimeToPeriod(t *testing.T) {
	cases := []struct {
		name      string
		t         time.Time
		periodSec int64
		want      time.Time
	}{
		{
			name:      "already aligned",
			t:         time.Unix(300, 0).UTC(),
			periodSec: 60,
			want:      time.Unix(300, 0).UTC(),
		},
		{
			name:      "truncates to boundary",
			t:         time.Unix(359, 0).UTC(),
			periodSec: 60,
			want:      time.Unix(300, 0).UTC(),
		},
		{
			name:      "zero period returns input unchanged",
			t:         time.Unix(123, 456).UTC(),
			periodSec: 0,
			want:      time.Unix(123, 456).UTC(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, alignTimeToPeriod(tc.t, tc.periodSec))
		})
	}
}

func TestResourceKey(t *testing.T) {
	cases := []struct {
		name       string
		namespace  string
		dimensions map[string]string
		want       string
	}{
		{
			name:      "no dimensions",
			namespace: "AWS/EC2",
			want:      "AWS/EC2",
		},
		{
			name:       "single dimension",
			namespace:  "AWS/EC2",
			dimensions: map[string]string{"InstanceId": "i-1234"},
			want:       "AWS/EC2,InstanceId=i-1234",
		},
		{
			name:       "multiple dimensions sorted",
			namespace:  "AWS/EC2",
			dimensions: map[string]string{"Z": "z", "A": "a"},
			want:       "AWS/EC2,A=a,Z=z",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, resourceKey(tc.namespace, tc.dimensions))
		})
	}
}

func TestToServiceAttributes(t *testing.T) {
	cases := []struct {
		namespace string
		wantNS    string
		wantName  string
	}{
		{"AWS/EC2", "AWS", "EC2"},
		{"AWS/ApplicationELB", "AWS", "ApplicationELB"},
		{"CustomNamespace", "", "CustomNamespace"},
		{"aws/ec2", "aws", "ec2"}, // case-insensitive match on provider prefix
	}
	for _, tc := range cases {
		t.Run(tc.namespace, func(t *testing.T) {
			ns, name := toServiceAttributes(tc.namespace)
			require.Equal(t, tc.wantNS, ns)
			require.Equal(t, tc.wantName, name)
		})
	}
}

// --- conversion tests ---

func TestConvertGetMetricDataToPdata_Empty(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)
	md := scr.convertGetMetricDataToPdata(nil, nil, time.Now())
	require.Equal(t, 0, md.ResourceMetrics().Len())
}

func TestConvertGetMetricDataToPdata_SkipsNoValues(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)
	results := []types.MetricDataResult{
		{Id: aws.String("q0"), Values: nil}, // no values – should be skipped
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
	}
	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 0, md.ResourceMetrics().Len())
}

func TestConvertGetMetricDataToPdata_SingleMetric(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	results := []types.MetricDataResult{
		{
			Id:         aws.String("q0"),
			Values:     []float64{42.5},
			Timestamps: []time.Time{ts},
		},
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Dimensions: map[string]string{"InstanceId": "i-abc"}},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 1, md.ResourceMetrics().Len())

	rm := md.ResourceMetrics().At(0)
	attrs := rm.Resource().Attributes()
	v, ok := attrs.Get("cloud.provider")
	require.True(t, ok)
	require.Equal(t, "aws", v.Str())
	v, ok = attrs.Get("cloud.region")
	require.True(t, ok)
	require.Equal(t, "us-east-1", v.Str())
	v, ok = attrs.Get("service.name")
	require.True(t, ok)
	require.Equal(t, "EC2", v.Str())
	v, ok = attrs.Get("service.namespace")
	require.True(t, ok)
	require.Equal(t, "AWS", v.Str())
	// InstanceId maps to service.instance.id
	v, ok = attrs.Get("service.instance.id")
	require.True(t, ok)
	require.Equal(t, "i-abc", v.Str())

	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, metadata.ScopeName, sm.Scope().Name())
	require.Equal(t, 1, sm.Metrics().Len())

	metric := sm.Metrics().At(0)
	require.Equal(t, "cpu_utilization", metric.Name()) // snake_case
	require.Equal(t, 1, metric.Gauge().DataPoints().Len())
	dp := metric.Gauge().DataPoints().At(0)
	require.InDelta(t, 42.5, dp.DoubleValue(), 0.001)
}

func TestConvertGetMetricDataToPdata_NonSemconvDimensionsSnakeCase(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	results := []types.MetricDataResult{
		{Id: aws.String("q0"), Values: []float64{1.0}, Timestamps: []time.Time{ts}},
	}
	batch := []MetricQuery{
		{Namespace: "AWS/ApplicationELB", MetricName: "RequestCount", Dimensions: map[string]string{
			"LoadBalancer": "app/my-lb/abc123",
			"TargetGroup":  "targetgroup/my-tg/def456",
		}},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 1, md.ResourceMetrics().Len())
	attrs := md.ResourceMetrics().At(0).Resource().Attributes()

	// Non-semconv dimensions must be snake_cased, not emitted as PascalCase.
	_, hasPascal := attrs.Get("LoadBalancer")
	require.False(t, hasPascal, "dimension key should be snake_case, not PascalCase")
	v, ok := attrs.Get("load_balancer")
	require.True(t, ok)
	require.Equal(t, "app/my-lb/abc123", v.Str())
	v, ok = attrs.Get("target_group")
	require.True(t, ok)
	require.Equal(t, "targetgroup/my-tg/def456", v.Str())
}

func TestConvertGetMetricDataToPdata_GroupsSameResource(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	dims := map[string]string{"InstanceId": "i-abc"}
	results := []types.MetricDataResult{
		{Id: aws.String("q0"), Values: []float64{1.0}, Timestamps: []time.Time{ts}},
		{Id: aws.String("q1"), Values: []float64{2.0}, Timestamps: []time.Time{ts}},
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Dimensions: dims},
		{Namespace: "AWS/EC2", MetricName: "NetworkIn", Dimensions: dims},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	// Both metrics share the same namespace+dimensions, so only one ResourceMetrics.
	require.Equal(t, 1, md.ResourceMetrics().Len())
	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 2, sm.Metrics().Len())
	names := []string{sm.Metrics().At(0).Name(), sm.Metrics().At(1).Name()}
	require.ElementsMatch(t, []string{"cpu_utilization", "network_in"}, names)
}

func TestConvertGetMetricDataToPdata_DifferentResourcesPerNamespace(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	results := []types.MetricDataResult{
		{Id: aws.String("q0"), Values: []float64{1.0}, Timestamps: []time.Time{ts}},
		{Id: aws.String("q1"), Values: []float64{2.0}, Timestamps: []time.Time{ts}},
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Dimensions: map[string]string{"InstanceId": "i-1"}},
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Dimensions: map[string]string{"InstanceId": "i-2"}},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 2, md.ResourceMetrics().Len())
}

func TestConvertGetMetricDataToPdata_FallbackTimestamp(t *testing.T) {
	endTime := time.Unix(1_700_000_300, 0).UTC()
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	results := []types.MetricDataResult{
		// Timestamps slice is shorter than Values slice.
		{Id: aws.String("q0"), Values: []float64{1.0, 2.0}, Timestamps: []time.Time{}},
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, endTime)
	dps := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
	require.Equal(t, 2, dps.Len())
	// Both data points should fall back to endTime.
	for i := 0; i < dps.Len(); i++ {
		require.Equal(t, endTime.UnixNano(), int64(dps.At(i).Timestamp()))
	}
}

// --- listMetrics tests ---

func TestListMetrics_SinglePage(t *testing.T) {
	mc := &mockMetricsClient{}
	mc.On("ListMetrics", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.ListMetricsOutput{
			Metrics: []types.Metric{
				{Namespace: aws.String("AWS/EC2"), MetricName: aws.String("CPUUtilization")},
			},
			NextToken: nil,
		}, nil,
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{
		Discovery: &MetricsDiscoveryConfig{Limit: 10, Stat: "Average"},
	}}
	scr := testScraper(cfg)
	scr.client = mc

	out, err := scr.listMetrics(t.Context())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "AWS/EC2", out[0].Namespace)
	require.Equal(t, "CPUUtilization", out[0].MetricName)
	mc.AssertExpectations(t)
}

func TestListMetrics_Paginated(t *testing.T) {
	token := "next-page"
	mc := &mockMetricsClient{}
	mc.On("ListMetrics", mock.Anything, mock.MatchedBy(func(p *cloudwatch.ListMetricsInput) bool {
		return p.NextToken == nil
	}), mock.Anything).Return(
		&cloudwatch.ListMetricsOutput{
			Metrics:   []types.Metric{{Namespace: aws.String("AWS/EC2"), MetricName: aws.String("CPUUtilization")}},
			NextToken: &token,
		}, nil,
	).Once()
	mc.On("ListMetrics", mock.Anything, mock.MatchedBy(func(p *cloudwatch.ListMetricsInput) bool {
		return p.NextToken != nil && *p.NextToken == token
	}), mock.Anything).Return(
		&cloudwatch.ListMetricsOutput{
			Metrics:   []types.Metric{{Namespace: aws.String("AWS/EC2"), MetricName: aws.String("NetworkIn")}},
			NextToken: nil,
		}, nil,
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{
		Discovery: &MetricsDiscoveryConfig{Limit: 100, Stat: "Average"},
	}}
	scr := testScraper(cfg)
	scr.client = mc

	out, err := scr.listMetrics(t.Context())
	require.NoError(t, err)
	require.Len(t, out, 2)
	mc.AssertExpectations(t)
}

func TestListMetrics_LimitRespected(t *testing.T) {
	mc := &mockMetricsClient{}
	mc.On("ListMetrics", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.ListMetricsOutput{
			Metrics: []types.Metric{
				{Namespace: aws.String("AWS/EC2"), MetricName: aws.String("M1")},
				{Namespace: aws.String("AWS/EC2"), MetricName: aws.String("M2")},
				{Namespace: aws.String("AWS/EC2"), MetricName: aws.String("M3")},
			},
			NextToken: nil,
		}, nil,
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{
		Discovery: &MetricsDiscoveryConfig{Limit: 2, Stat: "Average"},
	}}
	scr := testScraper(cfg)
	scr.client = mc

	out, err := scr.listMetrics(t.Context())
	require.NoError(t, err)
	require.Len(t, out, 2)
}

func TestListMetrics_Error(t *testing.T) {
	mc := &mockMetricsClient{}
	mc.On("ListMetrics", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.ListMetricsOutput{}, errors.New("API error"),
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{
		Discovery: &MetricsDiscoveryConfig{Limit: 10, Stat: "Average"},
	}}
	scr := testScraper(cfg)
	scr.client = mc

	_, err := scr.listMetrics(t.Context())
	require.Error(t, err)
}

// --- pollBatch tests ---

func TestPollBatch_SinglePage(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()
	mc := &mockMetricsClient{}
	mc.On("GetMetricData", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{Id: aws.String("q0"), Values: []float64{99.9}, Timestamps: []time.Time{ts}},
			},
			NextToken: nil,
		}, nil,
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{Period: 60 * time.Second}}
	scr := testScraper(cfg)
	scr.client = mc

	batch := []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Stat: "Average"}}
	md, err := scr.pollBatch(t.Context(), batch, ts.Add(-60*time.Second), ts)
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	mc.AssertExpectations(t)
}

func TestPollBatch_Paginated(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()
	token := "page2"
	mc := &mockMetricsClient{}
	mc.On("GetMetricData", mock.Anything, mock.MatchedBy(func(p *cloudwatch.GetMetricDataInput) bool {
		return p.NextToken == nil
	}), mock.Anything).Return(
		&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{Id: aws.String("q0"), Values: []float64{1.0}, Timestamps: []time.Time{ts}},
			},
			NextToken: &token,
		}, nil,
	).Once()
	mc.On("GetMetricData", mock.Anything, mock.MatchedBy(func(p *cloudwatch.GetMetricDataInput) bool {
		return p.NextToken != nil && *p.NextToken == token
	}), mock.Anything).Return(
		&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{Id: aws.String("q0"), Values: []float64{2.0}, Timestamps: []time.Time{ts.Add(60 * time.Second)}},
			},
			NextToken: nil,
		}, nil,
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{Period: 60 * time.Second}}
	scr := testScraper(cfg)
	scr.client = mc

	batch := []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Stat: "Average"}}
	md, err := scr.pollBatch(t.Context(), batch, ts.Add(-120*time.Second), ts)
	require.NoError(t, err)
	// Both pages for the same metric/resource → 1 ResourceMetrics, 1 metric, 2 data points.
	require.Equal(t, 1, md.ResourceMetrics().Len())
	dps := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
	require.Equal(t, 2, dps.Len())
	mc.AssertExpectations(t)
}

func TestPollBatch_Error(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()
	mc := &mockMetricsClient{}
	mc.On("GetMetricData", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.GetMetricDataOutput{}, errors.New("throttled"),
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{Period: 60 * time.Second}}
	scr := testScraper(cfg)
	scr.client = mc

	batch := []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}}
	_, err := scr.pollBatch(t.Context(), batch, ts.Add(-60*time.Second), ts)
	require.ErrorContains(t, err, "GetMetricData")
	require.ErrorContains(t, err, "throttled")
}

// --- scrape integration tests ---

func TestScrape_ExplicitMetrics(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()
	mc := &mockMetricsClient{}
	mc.On("GetMetricData", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{Id: aws.String("q0"), Values: []float64{50.0}, Timestamps: []time.Time{ts}},
			},
			NextToken: nil,
		}, nil,
	)

	cfg := &Config{
		Region: "us-east-1",
		Metrics: MetricsConfig{
			Period: 60 * time.Second,
			Metrics: []MetricQuery{
				{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Stat: "Average"},
			},
		},
	}
	scr := testScraper(cfg)
	scr.client = mc

	md, err := scr.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
}

func TestScrape_Discovery(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()
	mc := &mockMetricsClient{}
	mc.On("ListMetrics", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.ListMetricsOutput{
			Metrics:   []types.Metric{{Namespace: aws.String("AWS/EC2"), MetricName: aws.String("CPUUtilization")}},
			NextToken: nil,
		}, nil,
	)
	mc.On("GetMetricData", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{Id: aws.String("q0"), Values: []float64{75.0}, Timestamps: []time.Time{ts}},
			},
			NextToken: nil,
		}, nil,
	)

	cfg := &Config{
		Region: "us-east-1",
		Metrics: MetricsConfig{
			Period: 60 * time.Second,
			Discovery: &MetricsDiscoveryConfig{
				Namespace: "AWS/EC2",
				Limit:     10,
				Stat:      "Average",
			},
		},
	}
	scr := testScraper(cfg)
	scr.client = mc

	md, err := scr.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	mc.AssertExpectations(t)
}

func TestScrape_DiscoveryEmpty(t *testing.T) {
	mc := &mockMetricsClient{}
	mc.On("ListMetrics", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.ListMetricsOutput{Metrics: nil, NextToken: nil}, nil,
	)

	cfg := &Config{
		Region: "us-east-1",
		Metrics: MetricsConfig{
			Period:    60 * time.Second,
			Discovery: &MetricsDiscoveryConfig{Limit: 10, Stat: "Average"},
		},
	}
	scr := testScraper(cfg)
	scr.client = mc

	md, err := scr.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 0, md.ResourceMetrics().Len())
}

func TestScrape_DiscoveryError(t *testing.T) {
	mc := &mockMetricsClient{}
	mc.On("ListMetrics", mock.Anything, mock.Anything, mock.Anything).Return(
		&cloudwatch.ListMetricsOutput{}, errors.New("list error"),
	)

	cfg := &Config{
		Region: "us-east-1",
		Metrics: MetricsConfig{
			Period:    60 * time.Second,
			Discovery: &MetricsDiscoveryConfig{Limit: 10, Stat: "Average"},
		},
	}
	scr := testScraper(cfg)
	scr.client = mc

	_, err := scr.scrape(t.Context())
	require.ErrorContains(t, err, "list metrics")
}
