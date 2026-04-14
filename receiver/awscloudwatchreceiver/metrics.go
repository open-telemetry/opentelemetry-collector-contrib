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
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

const (
	// dimensionInstanceID is the CloudWatch dimension name for EC2 instance; we map it to service.instance.id.
	dimensionInstanceID = "InstanceId"
	namespaceDelimiter  = "/"

	// maxGetMetricDataQueries is the AWS limit per GetMetricData request.
	maxGetMetricDataQueries = 500
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
	if cfg.Metrics.Discovery != nil {
		d := *cfg.Metrics.Discovery
		if d.Limit <= 0 {
			d.Limit = defaultMetricsDiscoverLimit
		}
		if d.Stat == "" {
			d.Stat = defaultMetricsStat
		}
		discovery = &d
	}
	return &cloudWatchMetricsScraper{
		settings:           settings,
		cfg:                cfg,
		period:             cfg.Metrics.Period,
		delay:              cfg.Metrics.Delay,
		collectionInterval: cfg.Metrics.CollectionInterval,
		metrics:            cfg.Metrics.Metrics,
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
	for batchStart := 0; batchStart < len(metricsToScrape); batchStart += maxGetMetricDataQueries {
		batchEnd := min(batchStart+maxGetMetricDataQueries, len(metricsToScrape))
		batch := metricsToScrape[batchStart:batchEnd]
		batchMd, err := s.pollBatch(ctx, batch, startTime, endTime)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		batchMd.ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
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
	if s.discovery.Namespace != "" {
		input.Namespace = aws.String(s.discovery.Namespace)
	}
	if s.discovery.MetricName != "" {
		input.MetricName = aws.String(s.discovery.MetricName)
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
				Stat:       s.discovery.Stat,
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

// pollBatch runs GetMetricData for a batch of metrics and returns the converted pdata.Metrics.
// It follows pagination via NextToken to collect all data points for the requested time window.
func (s *cloudWatchMetricsScraper) pollBatch(ctx context.Context, batch []MetricQuery, startTime, endTime time.Time) (pmetric.Metrics, error) {
	queries := make([]types.MetricDataQuery, 0, len(batch))
	for i, q := range batch {
		stat := q.Stat
		if stat == "" {
			stat = defaultMetricsStat
		}
		mdq := types.MetricDataQuery{
			Id: aws.String(fmt.Sprintf("q%d", i)),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Namespace:  aws.String(q.Namespace),
					MetricName: aws.String(q.MetricName),
					Dimensions: dimensionsFromMap(q.Dimensions),
				},
				Period: aws.Int32(int32(s.period.Seconds())),
				Stat:   aws.String(stat),
			},
		}
		queries = append(queries, mdq)
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

// resourceKey returns a stable string identifying a resource by namespace and dimensions,
// used to group metrics from the same resource under a single ResourceMetrics.
// Dimensions are sorted by key for determinism. "," and "=" are safe separators because
// AWS namespace and dimension names/values are restricted to [a-zA-Z0-9._\-#:/].
func resourceKey(namespace string, dimensions map[string]string) string {
	keys := make([]string, 0, len(dimensions))
	for k := range dimensions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, 1+len(keys))
	parts = append(parts, namespace)
	for _, k := range keys {
		parts = append(parts, k+"="+dimensions[k])
	}
	return strings.Join(parts, ",")
}

// toServiceAttributes splits the CloudWatch namespace into service.namespace and service.name
// E.g. "AWS/EC2" -> ("AWS", "EC2"); otherwise returns ("", namespace).
func toServiceAttributes(namespace string) (serviceNamespace, serviceName string) {
	before, after, ok := strings.Cut(namespace, namespaceDelimiter)
	if ok && strings.EqualFold(before, conventions.CloudProviderAWS.Value.AsString()) {
		return before, after
	}
	return "", namespace
}

// setResourceAttributes sets resource attributes from namespace and dimensions, reusing the same
// conventions (cloud.*, service.*, InstanceId -> service.instance.id).
func (s *cloudWatchMetricsScraper) setResourceAttributes(resource pcommon.Resource, namespace string, dimensions map[string]string) {
	attrs := resource.Attributes()
	attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attrs.PutStr(string(conventions.CloudRegionKey), s.cfg.Region)
	serviceNamespace, serviceName := toServiceAttributes(namespace)
	if serviceNamespace != "" {
		attrs.PutStr(string(conventions.ServiceNamespaceKey), serviceNamespace)
	}
	attrs.PutStr(string(conventions.ServiceNameKey), serviceName)
	for k, v := range dimensions {
		switch k {
		case dimensionInstanceID:
			attrs.PutStr(string(conventions.ServiceInstanceIDKey), v)
		default:
			attrs.PutStr(strcase.ToSnake(k), v)
		}
	}
}

// convertGetMetricDataToPdata converts GetMetricData results to pdata.Metrics. Resource attributes follow the
// same conventions (cloud.*, service.*, dimension mapping).
// Results are matched to metricsList by query Id ("q0", "q1", ...).
// Metrics sharing the same namespace and dimensions are grouped under a single ResourceMetrics.
func (s *cloudWatchMetricsScraper) convertGetMetricDataToPdata(results []types.MetricDataResult, metricsList []MetricQuery, endTime time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	scopeByKey := make(map[string]pmetric.ScopeMetrics)
	// metricByKey deduplicates metrics across paginated results: the same query ID
	// may appear in multiple GetMetricData pages, each carrying different data points.
	type metricKey struct{ resource, name string }
	metricByKey := make(map[metricKey]pmetric.Metric)

	// Precompute snake_case names once; the same query may appear across multiple pages.
	metricNames := make([]string, len(metricsList))
	for i, q := range metricsList {
		metricNames[i] = strcase.ToSnake(q.MetricName)
	}

	for _, result := range results {
		if len(result.Values) == 0 || result.Id == nil {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimPrefix(*result.Id, "q"))
		if err != nil || idx < 0 || idx >= len(metricsList) {
			continue
		}
		q := metricsList[idx]
		key := resourceKey(q.Namespace, q.Dimensions)
		sm, ok := scopeByKey[key]
		if !ok {
			rm := md.ResourceMetrics().AppendEmpty()
			s.setResourceAttributes(rm.Resource(), q.Namespace, q.Dimensions)
			sm = rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName(metadata.ScopeName)
			scopeByKey[key] = sm
		}
		metricName := metricNames[idx]
		mk := metricKey{key, metricName}
		metric, exists := metricByKey[mk]
		if !exists {
			metric = sm.Metrics().AppendEmpty()
			metric.SetName(metricName)
			metric.SetEmptyGauge()
			metricByKey[mk] = metric
		}

		for j, v := range result.Values {
			dp := metric.Gauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(v)
			if j < len(result.Timestamps) {
				dp.SetTimestamp(pcommon.Timestamp(result.Timestamps[j].UnixNano()))
			} else {
				dp.SetTimestamp(pcommon.NewTimestampFromTime(endTime))
			}
		}
	}
	return md
}

