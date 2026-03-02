// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
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

type metricsReceiver struct {
	settings     receiver.Settings
	region       string
	profile      string
	imdsEndpoint string
	pollInterval time.Duration
	period       time.Duration
	metrics      []MetricQuery           // explicit list when not using discovery
	discovery    *MetricsDiscoveryConfig // when set, we call ListMetrics each poll
	consumer     consumer.Metrics
	client       metricsClient
	wg           sync.WaitGroup
	doneChan     chan struct{}
}

type metricsClient interface {
	ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

func newMetricsReceiver(cfg *Config, settings receiver.Settings, consumer consumer.Metrics) *metricsReceiver {
	period := cfg.Metrics.Period
	if period == 0 {
		period = defaultMetricsPeriod
	}
	pollInterval := cfg.Metrics.PollInterval
	if pollInterval == 0 {
		pollInterval = defaultMetricsPoll
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
	return &metricsReceiver{
		settings:     settings,
		region:       cfg.Region,
		profile:      cfg.Profile,
		imdsEndpoint: cfg.IMDSEndpoint,
		pollInterval: pollInterval,
		period:       period,
		metrics:      cfg.Metrics.Metrics,
		discovery:    discovery,
		consumer:     consumer,
		doneChan:     make(chan struct{}),
	}
}

func (m *metricsReceiver) Start(ctx context.Context, _ component.Host) error {
	hasExplicit := len(m.metrics) > 0
	hasDiscovery := m.discovery != nil
	m.settings.Logger.Debug("metrics receiver Start() called",
		zap.Int("configured_metrics", len(m.metrics)),
		zap.Bool("discovery_enabled", hasDiscovery))
	if !hasExplicit && !hasDiscovery {
		m.settings.Logger.Info("no metrics or discovery configured, metrics poller will not run")
		return nil
	}
	m.settings.Logger.Debug("starting CloudWatch metrics poller", zap.Duration("poll_interval", m.pollInterval))
	m.wg.Add(1)
	go m.pollLoop(ctx)
	return nil
}

func (m *metricsReceiver) Shutdown(_ context.Context) error {
	m.settings.Logger.Debug("shutting down metrics receiver")
	close(m.doneChan)
	m.wg.Wait()
	return nil
}

func (m *metricsReceiver) pollLoop(ctx context.Context) {
	defer m.wg.Done()
	if err := m.poll(ctx); err != nil {
		m.settings.Logger.Error("initial metrics poll failed", zap.Error(err))
	}
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.doneChan:
			return
		case <-ticker.C:
			if err := m.poll(ctx); err != nil {
				m.settings.Logger.Error("metrics poll failed", zap.Error(err))
			}
		}
	}
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

func (m *metricsReceiver) poll(ctx context.Context) error {
	m.settings.Logger.Debug("polling CloudWatch metrics", zap.String("region", m.region))
	if err := m.ensureClient(); err != nil {
		return err
	}

	var metricsToScrape []MetricQuery
	if m.discovery != nil {
		discovered, err := m.listMetrics(ctx)
		if err != nil {
			return fmt.Errorf("list metrics: %w", err)
		}
		metricsToScrape = discovered
		m.settings.Logger.Debug("discovered metrics", zap.Int("count", len(metricsToScrape)))
		if len(metricsToScrape) == 0 {
			return nil
		}
	} else {
		metricsToScrape = m.metrics
	}

	periodSec := int64(m.period.Seconds())
	now := time.Now().UTC()
	// Align EndTime to period boundary for better performance (GetMetricData docs).
	endTime := alignTimeToPeriod(now, periodSec)
	lookback := time.Duration(lookbackMultiplier) * m.period
	startTime := alignTimeToPeriod(endTime.Add(-lookback), periodSec)

	// GetMetricData allows up to 500 queries per request; batch if needed.
	for batchStart := 0; batchStart < len(metricsToScrape); batchStart += maxGetMetricDataQueries {
		batchEnd := batchStart + maxGetMetricDataQueries
		if batchEnd > len(metricsToScrape) {
			batchEnd = len(metricsToScrape)
		}
		batch := metricsToScrape[batchStart:batchEnd]
		if err := m.pollBatch(ctx, batch, startTime, endTime); err != nil {
			return err
		}
	}
	return nil
}

// listMetrics discovers metrics via ListMetrics API, respecting discovery config (namespace, metric name, limit).
func (m *metricsReceiver) listMetrics(ctx context.Context) ([]MetricQuery, error) {
	limit := m.discovery.Limit
	if limit <= 0 {
		limit = defaultMetricsDiscoverLimit
	}
	stat := m.discovery.Stat
	if stat == "" {
		stat = defaultMetricsStat
	}

	input := &cloudwatch.ListMetricsInput{}
	if m.discovery.Namespace != "" {
		input.Namespace = aws.String(m.discovery.Namespace)
	}
	if m.discovery.MetricName != "" {
		input.MetricName = aws.String(m.discovery.MetricName)
	}

	var out []MetricQuery
	var nextToken *string
	for {
		input.NextToken = nextToken
		resp, err := m.client.ListMetrics(ctx, input)
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

// pollBatch runs GetMetricData for a batch of metrics and sends the result to the consumer.
func (m *metricsReceiver) pollBatch(ctx context.Context, batch []MetricQuery, startTime, endTime time.Time) error {
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
				Period: aws.Int32(int32(m.period.Seconds())),
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

	out, err := m.client.GetMetricData(ctx, input)
	if err != nil {
		return fmt.Errorf("GetMetricData: %w", err)
	}

	withData := 0
	for _, r := range out.MetricDataResults {
		if len(r.Values) > 0 {
			withData++
		}
	}
	m.settings.Logger.Debug("GetMetricData response",
		zap.Int("results", len(out.MetricDataResults)),
		zap.Int("results_with_datapoints", withData),
		zap.Time("start", startTime),
		zap.Time("end", endTime))

	md := m.convertGetMetricDataToPdata(out, batch, endTime)
	if md.ResourceMetrics().Len() == 0 {
		if withData == 0 && len(batch) > 0 {
			m.settings.Logger.Debug("No metric data in response; CloudWatch often has 2-5 min delay",
				zap.String("region", m.region))
		}
		return nil
	}
	totalDP := 0
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				totalDP += sm.Metrics().At(k).Gauge().DataPoints().Len()
			}
		}
	}
	m.settings.Logger.Debug("sending metrics to consumer",
		zap.Int("resource_metrics", md.ResourceMetrics().Len()),
		zap.Int("data_points", totalDP))
	return m.consumer.ConsumeMetrics(ctx, md)
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
func (m *metricsReceiver) setResourceAttributes(resource pcommon.Resource, namespace string, dimensions map[string]string) {
	attrs := resource.Attributes()
	attrs.PutStr(string(semconv.CloudProviderKey), semconv.CloudProviderAWS.Value.AsString())
	attrs.PutStr(string(semconv.CloudRegionKey), m.region)
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
func (m *metricsReceiver) convertGetMetricDataToPdata(out *cloudwatch.GetMetricDataOutput, metricsList []MetricQuery, endTime time.Time) pmetric.Metrics {
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
		m.setResourceAttributes(rm.Resource(), q.Namespace, q.Dimensions)

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

func (m *metricsReceiver) ensureClient() error {
	m.settings.Logger.Debug("ensuring CloudWatch client", zap.String("region", m.region))
	if m.client != nil {
		return nil
	}
	opts := []func(*config.LoadOptions) error{config.WithRegion(m.region)}
	if m.imdsEndpoint != "" {
		opts = append(opts, config.WithEC2IMDSEndpoint(m.imdsEndpoint))
	}
	if m.profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(m.profile))
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return err
	}
	m.client = cloudwatch.NewFromConfig(cfg)
	m.settings.Logger.Debug("CloudWatch client initialized", zap.String("region", m.region))
	return nil
}
