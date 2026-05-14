// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

const (
	// maxGetMetricDataQueries is the AWS limit per GetMetricData request.
	maxGetMetricDataQueries = 500

	// statsPerMetric is the number of CloudWatch statistics we request per metric (Sum, SampleCount, Minimum, Maximum).
	statsPerMetric = 4

	// stat index constants within a metric's sub-queries
	statIdxSum   = 0
	statIdxCount = 1
	statIdxMin   = 2
	statIdxMax   = 3
)

type cloudWatchMetricsScraper struct {
	settings           receiver.Settings
	cfg                *Config
	period             time.Duration
	delay              time.Duration
	collectionInterval time.Duration
	metrics            []MetricQuery
	discovery          *MetricsDiscoveryConfig
	client             metricsClient
}

type metricsClient interface {
	ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

func newCloudWatchMetricsScraper(cfg *Config, settings receiver.Settings) *cloudWatchMetricsScraper {
	var discovery *MetricsDiscoveryConfig
	if d := cfg.Metrics.Discovery; d != nil {
		discoveryCfg := *d
		if discoveryCfg.Limit <= 0 {
			discoveryCfg.Limit = defaultMetricsDiscoverLimit
		}
		discovery = &discoveryCfg
	}
	return &cloudWatchMetricsScraper{
		settings:           settings,
		cfg:                cfg,
		period:             cfg.Metrics.Period,
		delay:              cfg.Metrics.Delay,
		collectionInterval: cfg.Metrics.CollectionInterval,
		metrics:            cfg.Metrics.Queries,
		discovery:          discovery,
	}
}

func (s *cloudWatchMetricsScraper) start(ctx context.Context, _ component.Host) error {
	s.settings.Logger.Debug("initializing CloudWatch client", zap.String("region", s.cfg.Region))
	opts := []func(*config.LoadOptions) error{config.WithRegion(s.cfg.Region)}
	if s.cfg.IMDSEndpoint != "" {
		opts = append(opts, config.WithEC2IMDSEndpoint(s.cfg.IMDSEndpoint))
	}
	if s.cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(s.cfg.Profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return err
	}
	s.client = cloudwatch.NewFromConfig(cfg)
	return nil
}

func (s *cloudWatchMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.settings.Logger.Debug("scraping CloudWatch metrics", zap.String("region", s.cfg.Region))

	var metricsToScrape []MetricQuery
	if s.discovery != nil {
		discovered, err := s.listMetrics(ctx)
		if err != nil {
			return pmetric.NewMetrics(), fmt.Errorf("list metrics: %w", err)
		}
		metricsToScrape = discovered
		s.settings.Logger.Debug("discovered metrics", zap.Int("count", len(metricsToScrape)))
		if len(metricsToScrape) == 0 {
			return pmetric.NewMetrics(), nil
		}
	} else {
		metricsToScrape = s.metrics
	}

	periodSec := int64(s.period.Seconds())
	now := time.Now().UTC()
	// Shift endTime back by delay to account for CloudWatch metric publication latency.
	// Query exactly one collectionInterval worth of data so consecutive scrapes do not overlap.
	endTime := alignTimeToPeriod(now.Add(-s.delay), periodSec)
	startTime := alignTimeToPeriod(endTime.Add(-s.collectionInterval), periodSec)

	md := pmetric.NewMetrics()
	for batchStart := 0; batchStart < len(metricsToScrape); {
		// Build the largest batch whose total sub-query count stays within the API limit.
		queryCount := numSubQueries(metricsToScrape[batchStart])
		batchEnd := batchStart + 1
		for batchEnd < len(metricsToScrape) {
			n := numSubQueries(metricsToScrape[batchEnd])
			if queryCount+n > maxGetMetricDataQueries {
				break
			}
			queryCount += n
			batchEnd++
		}
		batch := metricsToScrape[batchStart:batchEnd]
		batchMd, err := s.pollBatch(ctx, batch, startTime, endTime)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		batchMd.ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
		batchStart = batchEnd
	}
	return md, nil
}

// alignTimeToPeriod rounds t down to the nearest period boundary (in seconds from Unix epoch).
// Per GetMetricData docs, aligning StartTime and EndTime to the metric's Period improves
// performance: https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html
func alignTimeToPeriod(t time.Time, periodSec int64) time.Time {
	if periodSec <= 0 {
		return t
	}
	unix := t.Unix()
	aligned := (unix / periodSec) * periodSec
	return time.Unix(aligned, 0).UTC()
}

// listMetrics discovers metrics via ListMetrics API, respecting discovery config (namespace, metric name, limit).
func (s *cloudWatchMetricsScraper) listMetrics(ctx context.Context) ([]MetricQuery, error) {
	input := &cloudwatch.ListMetricsInput{}
	if f := s.discovery.Filters.Get(); f != nil {
		if f.Namespace != "" {
			input.Namespace = aws.String(f.Namespace)
		}
		if f.MetricName != "" {
			input.MetricName = aws.String(f.MetricName)
		}
	}

	var out []MetricQuery
	var nextToken *string
	for {
		input.NextToken = nextToken
		resp, err := s.client.ListMetrics(ctx, input)
		if err != nil {
			return nil, err
		}
		for _, met := range resp.Metrics {
			if len(out) >= s.discovery.Limit {
				return out, nil
			}
			q := MetricQuery{
				Namespace:  aws.ToString(met.Namespace),
				MetricName: aws.ToString(met.MetricName),
				Dimensions: dimensionsToMap(met.Dimensions),
				Stats:      s.discovery.Stats,
			}
			out = append(out, q)
		}
		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}

func dimensionsToMap(dims []types.Dimension) map[string]string {
	if len(dims) == 0 {
		return nil
	}
	out := make(map[string]string, len(dims))
	for _, d := range dims {
		if d.Name != nil && d.Value != nil {
			out[aws.ToString(d.Name)] = aws.ToString(d.Value)
		}
	}
	return out
}

// metricStats are the four CloudWatch statistics fetched for every metric to build a Summary.
var metricStats = [statsPerMetric]string{"Sum", "SampleCount", "Minimum", "Maximum"}

// numSubQueries returns the number of GetMetricData sub-queries needed for one metric.
// Metrics without an explicit Stats list use all four summary statistics.
func numSubQueries(q MetricQuery) int {
	if len(q.Stats) == 0 {
		return statsPerMetric
	}
	return len(q.Stats)
}

// pollBatch runs GetMetricData for a batch of metrics and returns the converted pdata.Metrics.
// Each metric generates four sub-queries (Sum, SampleCount, Minimum, Maximum) so that the results
// can be combined into an OpenTelemetry Summary metric aligned with the CloudWatch Metric Streams
// OpenTelemetry 1.0.0 format.
// It follows pagination via NextToken to collect all data points for the requested time window.
func (s *cloudWatchMetricsScraper) pollBatch(ctx context.Context, batch []MetricQuery, startTime, endTime time.Time) (pmetric.Metrics, error) {
	totalQueries := 0
	for _, q := range batch {
		totalQueries += numSubQueries(q)
	}
	queries := make([]types.MetricDataQuery, 0, totalQueries)
	for i, q := range batch {
		stats := metricStats[:]
		if len(q.Stats) > 0 {
			stats = q.Stats
		}
		for si, stat := range stats {
			queries = append(queries, types.MetricDataQuery{
				Id: aws.String(fmt.Sprintf("q%d_%d", i, si)),
				MetricStat: &types.MetricStat{
					Metric: &types.Metric{
						Namespace:  aws.String(q.Namespace),
						MetricName: aws.String(q.MetricName),
						Dimensions: dimensionsFromMap(q.Dimensions),
					},
					Period: aws.Int32(int32(s.period.Seconds())),
					Stat:   aws.String(stat),
				},
			})
		}
	}

	input := &cloudwatch.GetMetricDataInput{
		StartTime:         aws.Time(startTime),
		EndTime:           aws.Time(endTime),
		MetricDataQueries: queries,
	}

	var allResults []types.MetricDataResult
	for {
		out, err := s.client.GetMetricData(ctx, input)
		if err != nil {
			return pmetric.NewMetrics(), fmt.Errorf("GetMetricData: %w", err)
		}
		allResults = append(allResults, out.MetricDataResults...)
		if out.NextToken == nil {
			break
		}
		input.NextToken = out.NextToken
	}

	withData := 0
	for _, r := range allResults {
		if len(r.Values) > 0 {
			withData++
		}
	}
	s.settings.Logger.Debug("GetMetricData response",
		zap.Int("results", len(allResults)),
		zap.Int("results_with_datapoints", withData),
		zap.Time("start", startTime),
		zap.Time("end", endTime))

	md := s.convertGetMetricDataToPdata(allResults, batch, endTime)
	if md.ResourceMetrics().Len() == 0 {
		if withData > 0 {
			s.settings.Logger.Warn("GetMetricData returned datapoints but conversion produced no resource metrics; check Id matching",
				zap.String("region", s.cfg.Region))
		} else if len(batch) > 0 {
			s.settings.Logger.Debug("No metric data in response; CloudWatch often has 2-5 min delay",
				zap.String("region", s.cfg.Region))
		}
	}
	return md, nil
}

func dimensionsFromMap(d map[string]string) []types.Dimension {
	if len(d) == 0 {
		return nil
	}
	out := make([]types.Dimension, 0, len(d))
	for k, v := range d {
		out = append(out, types.Dimension{Name: aws.String(k), Value: aws.String(v)})
	}
	return out
}

// setResourceAttributes sets the resource-level attributes that identify the AWS source.
// Aligned with the CloudWatch Metric Streams OpenTelemetry 1.0.0 format: only cloud.provider
// and cloud.region are set on the resource; namespace and dimensions go on data points.
func (s *cloudWatchMetricsScraper) setResourceAttributes(resource pcommon.Resource) {
	attrs := resource.Attributes()
	attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attrs.PutStr(string(conventions.CloudRegionKey), s.cfg.Region)
}

// parseQueryID parses a sub-query ID of the form "q{metricIdx}_{statIdx}".
func parseQueryID(id string) (metricIdx, statIdx int, err error) {
	trimmed := strings.TrimPrefix(id, "q")
	parts := strings.SplitN(trimmed, "_", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid query ID %q", id)
	}
	metricIdx, err = strconv.Atoi(parts[0])
	if err != nil {
		return metricIdx, statIdx, err
	}
	statIdx, err = strconv.Atoi(parts[1])
	return metricIdx, statIdx, err
}

// convertGetMetricDataToPdata converts GetMetricData results to pdata.Metrics.
//
// When a MetricQuery has no Stats (empty), all four statistics are fetched and combined into an
// OpenTelemetry Summary metric, aligned with the CloudWatch Metric Streams OpenTelemetry 1.0.0 format:
//   - Metric name: "amazonaws.com/{Namespace}/{MetricName}".
//   - Metric type: Summary with count (SampleCount), sum (Sum), min quantile (Minimum), max quantile (Maximum).
//   - Data point attributes: Namespace (string), MetricName (string), Dimensions (kvlist).
//
// When a MetricQuery has explicit Stats, each selected statistic is emitted as a Gauge data point on the
// same metric, with an additional "stat" attribute identifying the statistic.
func (s *cloudWatchMetricsScraper) convertGetMetricDataToPdata(results []types.MetricDataResult, metricsList []MetricQuery, endTime time.Time) pmetric.Metrics {
	// statMaps[metricIdx][statIdx] holds a map from timestamp → value for that metric/stat combination.
	// The inner slice length equals numSubQueries(metricsList[i]).
	type tsMap = map[time.Time]float64
	statMaps := make([][]tsMap, len(metricsList))
	for i, q := range metricsList {
		n := numSubQueries(q)
		statMaps[i] = make([]tsMap, n)
		for j := range statMaps[i] {
			statMaps[i][j] = make(tsMap)
		}
	}

	for _, result := range results {
		if result.Id == nil || len(result.Values) == 0 {
			continue
		}
		metricIdx, statIdx, err := parseQueryID(*result.Id)
		if err != nil || metricIdx < 0 || metricIdx >= len(metricsList) || statIdx < 0 || statIdx >= len(statMaps[metricIdx]) {
			continue
		}
		for j, v := range result.Values {
			ts := endTime
			if j < len(result.Timestamps) {
				ts = result.Timestamps[j]
			}
			statMaps[metricIdx][statIdx][ts] = v
		}
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	s.setResourceAttributes(rm.Resource())
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(metadata.ScopeName)

	periodNano := pcommon.Timestamp(s.period.Nanoseconds())

	for i, q := range metricsList {
		// Skip metrics with no data across any stat.
		hasData := false
		for _, tsm := range statMaps[i] {
			if len(tsm) > 0 {
				hasData = true
				break
			}
		}
		if !hasData {
			continue
		}

		metric := sm.Metrics().AppendEmpty()
		metric.SetName("amazonaws.com/" + q.Namespace + "/" + q.MetricName)

		if len(q.Stats) == 0 {
			// Summary mode: combine Sum, SampleCount, Minimum, Maximum into one Summary data point per timestamp.
			tsSet := make(map[time.Time]struct{})
			for _, tsm := range statMaps[i] {
				for ts := range tsm {
					tsSet[ts] = struct{}{}
				}
			}
			timestamps := make([]time.Time, 0, len(tsSet))
			for ts := range tsSet {
				timestamps = append(timestamps, ts)
			}
			sort.Slice(timestamps, func(a, b int) bool { return timestamps[a].Before(timestamps[b]) })

			summary := metric.SetEmptySummary()
			for _, ts := range timestamps {
				dp := summary.DataPoints().AppendEmpty()
				tsNano := pcommon.NewTimestampFromTime(ts)
				dp.SetTimestamp(tsNano)
				if tsNano > periodNano {
					dp.SetStartTimestamp(tsNano - periodNano)
				}
				if count, ok := statMaps[i][statIdxCount][ts]; ok {
					dp.SetCount(uint64(count))
				}
				if sum, ok := statMaps[i][statIdxSum][ts]; ok {
					dp.SetSum(sum)
				}
				if minVal, ok := statMaps[i][statIdxMin][ts]; ok {
					qv := dp.QuantileValues().AppendEmpty()
					qv.SetQuantile(0.0)
					qv.SetValue(minVal)
				}
				if maxVal, ok := statMaps[i][statIdxMax][ts]; ok {
					qv := dp.QuantileValues().AppendEmpty()
					qv.SetQuantile(1.0)
					qv.SetValue(maxVal)
				}
				applyQueryAttrs(dp.Attributes(), q)
			}
		} else {
			// Gauge mode: one Gauge data point per (stat, timestamp), tagged with a "stat" attribute.
			gauge := metric.SetEmptyGauge()
			for si, statName := range q.Stats {
				timestamps := make([]time.Time, 0, len(statMaps[i][si]))
				for ts := range statMaps[i][si] {
					timestamps = append(timestamps, ts)
				}
				sort.Slice(timestamps, func(a, b int) bool { return timestamps[a].Before(timestamps[b]) })
				for _, ts := range timestamps {
					dp := gauge.DataPoints().AppendEmpty()
					tsNano := pcommon.NewTimestampFromTime(ts)
					dp.SetTimestamp(tsNano)
					if tsNano > periodNano {
						dp.SetStartTimestamp(tsNano - periodNano)
					}
					dp.SetDoubleValue(statMaps[i][si][ts])
					applyQueryAttrs(dp.Attributes(), q)
					dp.Attributes().PutStr("stat", statName)
				}
			}
		}
	}

	// Drop the ResourceMetrics if no metric had any data.
	if sm.Metrics().Len() == 0 {
		return pmetric.NewMetrics()
	}
	return md
}

// applyQueryAttrs sets the common data point attributes: Namespace, MetricName, and Dimensions (kvlist).
func applyQueryAttrs(attrs pcommon.Map, q MetricQuery) {
	attrs.PutStr("Namespace", q.Namespace)
	attrs.PutStr("MetricName", q.MetricName)
	if len(q.Dimensions) > 0 {
		dimsMap := attrs.PutEmptyMap("Dimensions")
		for k, v := range q.Dimensions {
			dimsMap.PutStr(k, v)
		}
	}
}
