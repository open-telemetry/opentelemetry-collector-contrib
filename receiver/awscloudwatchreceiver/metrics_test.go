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
	"go.opentelemetry.io/collector/config/configoptional"
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

func TestParseQueryID(t *testing.T) {
	cases := []struct {
		id        string
		wantM     int
		wantS     int
		wantError bool
	}{
		{"q0_0", 0, 0, false},
		{"q5_3", 5, 3, false},
		{"q123_2", 123, 2, false},
		{"invalid", 0, 0, true},
		{"q_0", 0, 0, true},
		{"q0", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			m, s, err := parseQueryID(tc.id)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantM, m)
			require.Equal(t, tc.wantS, s)
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
		{Id: aws.String("q0_0"), Values: nil}, // no values – should be skipped
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
		{Id: aws.String("q0_0"), Values: []float64{20.0}, Timestamps: []time.Time{ts}}, // Sum
		{Id: aws.String("q0_1"), Values: []float64{3.0}, Timestamps: []time.Time{ts}},  // SampleCount
		{Id: aws.String("q0_2"), Values: []float64{0.0}, Timestamps: []time.Time{ts}},  // Minimum
		{Id: aws.String("q0_3"), Values: []float64{18.0}, Timestamps: []time.Time{ts}}, // Maximum
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Dimensions: map[string]string{"InstanceId": "i-abc"}},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 1, md.ResourceMetrics().Len())

	rm := md.ResourceMetrics().At(0)
	attrs := rm.Resource().Attributes()

	// Resource has only cloud.provider and cloud.region.
	v, ok := attrs.Get("cloud.provider")
	require.True(t, ok)
	require.Equal(t, "aws", v.Str())
	v, ok = attrs.Get("cloud.region")
	require.True(t, ok)
	require.Equal(t, "us-east-1", v.Str())
	_, hasServiceName := attrs.Get("service.name")
	require.False(t, hasServiceName, "service.name must not be on the resource")
	_, hasServiceNS := attrs.Get("service.namespace")
	require.False(t, hasServiceNS, "service.namespace must not be on the resource")

	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, metadata.ScopeName, sm.Scope().Name())
	require.Equal(t, 1, sm.Metrics().Len())

	metric := sm.Metrics().At(0)
	require.Equal(t, "amazonaws.com/AWS/EC2/CPUUtilization", metric.Name())

	// Must be Summary, not Gauge.
	require.Equal(t, 1, metric.Summary().DataPoints().Len())
	dp := metric.Summary().DataPoints().At(0)
	require.Equal(t, uint64(3), dp.Count())
	require.InDelta(t, 20.0, dp.Sum(), 0.001)

	// Quantile values: min (0.0) and max (1.0).
	require.Equal(t, 2, dp.QuantileValues().Len())
	require.Equal(t, 0.0, dp.QuantileValues().At(0).Quantile())
	require.InDelta(t, 0.0, dp.QuantileValues().At(0).Value(), 0.001)
	require.Equal(t, 1.0, dp.QuantileValues().At(1).Quantile())
	require.InDelta(t, 18.0, dp.QuantileValues().At(1).Value(), 0.001)

	// Data point attributes: Namespace, MetricName, Dimensions kvlist.
	dpAttrs := dp.Attributes()
	ns, ok := dpAttrs.Get("Namespace")
	require.True(t, ok)
	require.Equal(t, "AWS/EC2", ns.Str())
	mn, ok := dpAttrs.Get("MetricName")
	require.True(t, ok)
	require.Equal(t, "CPUUtilization", mn.Str())
	dims, ok := dpAttrs.Get("Dimensions")
	require.True(t, ok)
	instID, ok := dims.Map().Get("InstanceId")
	require.True(t, ok)
	require.Equal(t, "i-abc", instID.Str())
}

func TestConvertGetMetricDataToPdata_DimensionsPreserveCase(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	results := []types.MetricDataResult{
		{Id: aws.String("q0_1"), Values: []float64{1.0}, Timestamps: []time.Time{ts}}, // SampleCount only
	}
	batch := []MetricQuery{
		{Namespace: "AWS/ApplicationELB", MetricName: "RequestCount", Dimensions: map[string]string{
			"LoadBalancer": "app/my-lb/abc123",
			"TargetGroup":  "targetgroup/my-tg/def456",
		}},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 1, md.ResourceMetrics().Len())
	dp := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0)

	// Dimensions are stored as a kvlistValue with original CloudWatch casing.
	dims, ok := dp.Attributes().Get("Dimensions")
	require.True(t, ok)
	lb, ok := dims.Map().Get("LoadBalancer")
	require.True(t, ok)
	require.Equal(t, "app/my-lb/abc123", lb.Str())
	tg, ok := dims.Map().Get("TargetGroup")
	require.True(t, ok)
	require.Equal(t, "targetgroup/my-tg/def456", tg.Str())
}

func TestConvertGetMetricDataToPdata_GroupsSameMetricAcrossPages(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	t1 := time.Unix(1_700_000_000, 0).UTC()
	t2 := t1.Add(60 * time.Second)

	// Two pages of Sum results for the same metric at different timestamps.
	results := []types.MetricDataResult{
		{Id: aws.String("q0_0"), Values: []float64{10.0, 20.0}, Timestamps: []time.Time{t1, t2}},
		{Id: aws.String("q0_1"), Values: []float64{2.0, 4.0}, Timestamps: []time.Time{t1, t2}},
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 1, md.ResourceMetrics().Len())
	metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, 2, metric.Summary().DataPoints().Len())
}

func TestConvertGetMetricDataToPdata_MultipleMetrics(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	results := []types.MetricDataResult{
		{Id: aws.String("q0_1"), Values: []float64{1.0}, Timestamps: []time.Time{ts}},
		{Id: aws.String("q1_1"), Values: []float64{2.0}, Timestamps: []time.Time{ts}},
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
		{Namespace: "AWS/EC2", MetricName: "NetworkIn"},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 1, md.ResourceMetrics().Len()) // single resource for all metrics
	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)
	require.Equal(t, 2, sm.Metrics().Len())
	names := []string{sm.Metrics().At(0).Name(), sm.Metrics().At(1).Name()}
	require.ElementsMatch(t, []string{"amazonaws.com/AWS/EC2/CPUUtilization", "amazonaws.com/AWS/EC2/NetworkIn"}, names)
}

// --- gauge mode tests ---

func TestConvertGetMetricDataToPdata_GaugeMode_SingleStat(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	results := []types.MetricDataResult{
		{Id: aws.String("q0_0"), Values: []float64{42.5}, Timestamps: []time.Time{ts}},
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Stats: []string{"Average"}},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 1, md.ResourceMetrics().Len())

	metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, "amazonaws.com/AWS/EC2/CPUUtilization", metric.Name())

	// Must be Gauge, not Summary.
	require.Equal(t, 1, metric.Gauge().DataPoints().Len())
	dp := metric.Gauge().DataPoints().At(0)
	require.InDelta(t, 42.5, dp.DoubleValue(), 0.001)

	// Data point attributes include "stat".
	ns, ok := dp.Attributes().Get("Namespace")
	require.True(t, ok)
	require.Equal(t, "AWS/EC2", ns.Str())
	stat, ok := dp.Attributes().Get("stat")
	require.True(t, ok)
	require.Equal(t, "Average", stat.Str())
}

func TestConvertGetMetricDataToPdata_GaugeMode_MultipleStats(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	results := []types.MetricDataResult{
		{Id: aws.String("q0_0"), Values: []float64{10.0}, Timestamps: []time.Time{ts}}, // Sum
		{Id: aws.String("q0_1"), Values: []float64{99.9}, Timestamps: []time.Time{ts}}, // p99
	}
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Stats: []string{"Sum", "p99"}},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	require.Equal(t, 1, md.ResourceMetrics().Len())

	metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	// Two data points: one per stat.
	require.Equal(t, 2, metric.Gauge().DataPoints().Len())

	statVals := map[string]float64{}
	for j := 0; j < metric.Gauge().DataPoints().Len(); j++ {
		dp := metric.Gauge().DataPoints().At(j)
		s, ok := dp.Attributes().Get("stat")
		require.True(t, ok)
		statVals[s.Str()] = dp.DoubleValue()
	}
	require.InDelta(t, 10.0, statVals["Sum"], 0.001)
	require.InDelta(t, 99.9, statVals["p99"], 0.001)
}

func TestConvertGetMetricDataToPdata_GaugeMode_NoDimensionsAttr(t *testing.T) {
	cfg := &Config{Region: "us-east-1"}
	scr := testScraper(cfg)

	ts := time.Unix(1_700_000_000, 0).UTC()
	results := []types.MetricDataResult{
		{Id: aws.String("q0_0"), Values: []float64{5.0}, Timestamps: []time.Time{ts}},
	}
	// No dimensions set — Dimensions attr must not appear.
	batch := []MetricQuery{
		{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Stats: []string{"Average"}},
	}

	md := scr.convertGetMetricDataToPdata(results, batch, time.Now())
	dp := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	_, hasDims := dp.Attributes().Get("Dimensions")
	require.False(t, hasDims)
}

func TestPollBatch_GaugeMode_GeneratesOneSubQueryPerStat(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()
	mc := &mockMetricsClient{}
	mc.On("GetMetricData", mock.Anything, mock.MatchedBy(func(p *cloudwatch.GetMetricDataInput) bool {
		// Stats: ["Sum", "p99"] → 2 sub-queries.
		return len(p.MetricDataQueries) == 2
	}), mock.Anything).Return(
		&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{Id: aws.String("q0_0"), Values: []float64{10.0}, Timestamps: []time.Time{ts}},
				{Id: aws.String("q0_1"), Values: []float64{99.0}, Timestamps: []time.Time{ts}},
			},
		}, nil,
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{Period: 60 * time.Second}}
	scr := testScraper(cfg)
	scr.client = mc

	batch := []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization", Stats: []string{"Sum", "p99"}}}
	md, err := scr.pollBatch(t.Context(), batch, ts.Add(-60*time.Second), ts)
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	require.Equal(t, 2, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().Len())
	mc.AssertExpectations(t)
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
		Discovery: &MetricsDiscoveryConfig{Limit: 10},
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
		Discovery: &MetricsDiscoveryConfig{Limit: 100},
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
		Discovery: &MetricsDiscoveryConfig{Limit: 2},
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
		Discovery: &MetricsDiscoveryConfig{Limit: 10},
	}}
	scr := testScraper(cfg)
	scr.client = mc

	_, err := scr.listMetrics(t.Context())
	require.Error(t, err)
}

// --- pollBatch tests ---

func TestPollBatch_GeneratesFourSubQueries(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0).UTC()
	mc := &mockMetricsClient{}
	mc.On("GetMetricData", mock.Anything, mock.MatchedBy(func(p *cloudwatch.GetMetricDataInput) bool {
		// Each metric generates 4 sub-queries (Sum, SampleCount, Minimum, Maximum).
		return len(p.MetricDataQueries) == 4
	}), mock.Anything).Return(
		&cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{Id: aws.String("q0_0"), Values: []float64{20.0}, Timestamps: []time.Time{ts}},
				{Id: aws.String("q0_1"), Values: []float64{3.0}, Timestamps: []time.Time{ts}},
				{Id: aws.String("q0_2"), Values: []float64{4.0}, Timestamps: []time.Time{ts}},
				{Id: aws.String("q0_3"), Values: []float64{9.0}, Timestamps: []time.Time{ts}},
			},
		}, nil,
	)

	cfg := &Config{Region: "us-east-1", Metrics: MetricsConfig{Period: 60 * time.Second}}
	scr := testScraper(cfg)
	scr.client = mc

	batch := []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}}
	md, err := scr.pollBatch(t.Context(), batch, ts.Add(-60*time.Second), ts)
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())

	dp := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0)
	require.Equal(t, uint64(3), dp.Count())
	require.InDelta(t, 20.0, dp.Sum(), 0.001)
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
				{Id: aws.String("q0_1"), Values: []float64{1.0}, Timestamps: []time.Time{ts}},
			},
		}, nil,
	)

	cfg := &Config{
		Region: "us-east-1",
		Metrics: MetricsConfig{
			Period: 60 * time.Second,
			Queries: []MetricQuery{
				{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
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
				{Id: aws.String("q0_1"), Values: []float64{75.0}, Timestamps: []time.Time{ts}},
			},
		}, nil,
	)

	cfg := &Config{
		Region: "us-east-1",
		Metrics: MetricsConfig{
			Period: 60 * time.Second,
			Discovery: &MetricsDiscoveryConfig{
				Filters: configoptional.Some(MetricsDiscoveryFilters{Namespace: "AWS/EC2"}),
				Limit:   10,
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
			Discovery: &MetricsDiscoveryConfig{Limit: 10},
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
			Discovery: &MetricsDiscoveryConfig{Limit: 10},
		},
	}
	scr := testScraper(cfg)
	scr.client = mc

	_, err := scr.scrape(t.Context())
	require.ErrorContains(t, err, "list metrics")
}
