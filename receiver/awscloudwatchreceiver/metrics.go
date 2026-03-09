// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"context"
	"fmt"
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
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"
)

const (
	// dimensionInstanceID is the CloudWatch dimension name for EC2 instance; we map it to service.instance.id.
	dimensionInstanceID = "InstanceId"
	namespaceDelimiter  = "/"
)

type cloudWatchMetricsScraper struct {
	settings     receiver.Settings
	cfg          *Config
	period       time.Duration
	metrics      []MetricQuery           // explicit list when not using discovery
	discovery    *MetricsDiscoveryConfig // when set, we call ListMetrics each scrape
	client       metricsClient
}

type metricsClient interface {
	ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

func newCloudWatchMetricsScraper(cfg *Config, settings receiver.Settings) *cloudWatchMetricsScraper {
	period := cfg.Metrics.Period
	if period == 0 {
		period = defaultMetricsPeriod
	}
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
		settings:  settings,
		cfg:       cfg,
		period:    period,
		metrics:   cfg.Metrics.Metrics,
		discovery: discovery,
	}
}

func (s *cloudWatchMetricsScraper) start(ctx context.Context, host component.Host) error {
	return s.ensureClient()
}

func (s *cloudWatchMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.settings.Logger.Debug("scraping CloudWatch metrics", zap.String("region", s.cfg.Region))
	if err := s.ensureClient(); err != nil {
		return pmetric.NewMetrics(), err
	}

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
	endTime := alignTimeToPeriod(now, periodSec)
	lookback := time.Duration(lookbackMultiplier) * s.period
	startTime := alignTimeToPeriod(endTime.Add(-lookback), periodSec)

	md := pmetric.NewMetrics()
	for batchStart := 0; batchStart < len(metricsToScrape); batchStart += maxGetMetricDataQueries {
		batchEnd := batchStart + maxGetMetricDataQueries
		if batchEnd > len(metricsToScrape) {
			batchEnd = len(metricsToScrape)
		}
		batch := metricsToScrape[batchStart:batchEnd]
		batchMd, err := s.pollBatch(ctx, batch, startTime, endTime)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		batchMd.ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	}
	return md, nil
}

// lookbackMultiplier is how many periods we look back. CloudWatch often has 2–5 minute
// delay for 1-minute metrics, so we query a longer window to get data.
// TODO: revisit this.
const lookbackMultiplier = 10

// maxGetMetricDataQueries is the AWS limit per GetMetricData request.
const maxGetMetricDataQueries = 500

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
	limit := s.discovery.Limit
	if limit <= 0 {
		limit = defaultMetricsDiscoverLimit
	}
	stat := s.discovery.Stat
	if stat == "" {
		stat = defaultMetricsStat
	}

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
			if len(out) >= limit {
				return out, nil
			}
			q := MetricQuery{
				Namespace:  aws.ToString(met.Namespace),
				MetricName: aws.ToString(met.MetricName),
				Dimensions: dimensionsToMap(met.Dimensions),
				Stat:       stat,
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

	out, err := s.client.GetMetricData(ctx, input)
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("GetMetricData: %w", err)
	}

	withData := 0
	for _, r := range out.MetricDataResults {
		if len(r.Values) > 0 {
			withData++
		}
	}
	s.settings.Logger.Debug("GetMetricData response",
		zap.Int("results", len(out.MetricDataResults)),
		zap.Int("results_with_datapoints", withData),
		zap.Time("start", startTime),
		zap.Time("end", endTime))

	md := s.convertGetMetricDataToPdata(out, batch, endTime)
	if md.ResourceMetrics().Len() == 0 && withData > 0 {
		s.settings.Logger.Warn("GetMetricData returned datapoints but conversion produced no resource metrics; check Id matching",
			zap.String("region", s.cfg.Region))
	}
	if md.ResourceMetrics().Len() == 0 && withData == 0 && len(batch) > 0 {
		s.settings.Logger.Debug("No metric data in response; CloudWatch often has 2-5 min delay",
			zap.String("region", s.cfg.Region))
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

// toServiceAttributes splits the CloudWatch namespace into service.namespace and service.name
// E.g. "AWS/EC2" -> ("AWS", "EC2"); otherwise returns ("", namespace).
func toServiceAttributes(namespace string) (serviceNamespace, serviceName string) {
	before, after, ok := strings.Cut(namespace, namespaceDelimiter)
	if ok && strings.EqualFold(before, semconv.CloudProviderAWS.Value.AsString()) {
		return before, after
	}
	return "", namespace
}

// setResourceAttributes sets resource attributes from namespace and dimensions, reusing the same
// conventions (cloud.*, service.*, InstanceId -> service.instance.id).
func (s *cloudWatchMetricsScraper) setResourceAttributes(resource pcommon.Resource, namespace string, dimensions map[string]string) {
	attrs := resource.Attributes()
	attrs.PutStr(string(semconv.CloudProviderKey), semconv.CloudProviderAWS.Value.AsString())
	attrs.PutStr(string(semconv.CloudRegionKey), s.cfg.Region)
	serviceNamespace, serviceName := toServiceAttributes(namespace)
	if serviceNamespace != "" {
		attrs.PutStr(string(semconv.ServiceNamespaceKey), serviceNamespace)
	}
	attrs.PutStr(string(semconv.ServiceNameKey), serviceName)
	for k, v := range dimensions {
		switch k {
		case dimensionInstanceID:
			attrs.PutStr(string(semconv.ServiceInstanceIDKey), v)
		default:
			attrs.PutStr(k, v)
		}
	}
}

// convertGetMetricDataToPdata converts GetMetricData API output to pdata.Metrics. Resource attributes follow the
// same conventions (cloud.*, service.*, dimension mapping).
// Results are matched to metricsList by query Id ("q0", "q1", ...).
func (s *cloudWatchMetricsScraper) convertGetMetricDataToPdata(out *cloudwatch.GetMetricDataOutput, metricsList []MetricQuery, endTime time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, result := range out.MetricDataResults {
		if len(result.Values) == 0 || result.Id == nil {
			continue
		}
		var idx int
		if _, err := fmt.Sscanf(*result.Id, "q%d", &idx); err != nil || idx < 0 || idx >= len(metricsList) {
			continue
		}
		q := metricsList[idx]
		rm := md.ResourceMetrics().AppendEmpty()
		s.setResourceAttributes(rm.Resource(), q.Namespace, q.Dimensions)

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("awscloudwatchreceiver")
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(q.MetricName) // TODO: should we flatten the metric name?
		metric.SetEmptyGauge()

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

func (s *cloudWatchMetricsScraper) ensureClient() error {
	s.settings.Logger.Debug("ensuring CloudWatch client", zap.String("region", s.cfg.Region))
	if s.client != nil {
		return nil
	}
	opts := []func(*config.LoadOptions) error{config.WithRegion(s.cfg.Region)}
	if s.cfg.IMDSEndpoint != "" {
		opts = append(opts, config.WithEC2IMDSEndpoint(s.cfg.IMDSEndpoint))
	}
	if s.cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(s.cfg.Profile))
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return err
	}
	s.client = cloudwatch.NewFromConfig(cfg)
	s.settings.Logger.Debug("CloudWatch client initialized", zap.String("region", s.cfg.Region))
	return nil
}
