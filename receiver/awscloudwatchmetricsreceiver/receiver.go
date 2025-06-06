// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricReceiver struct {
	region        string
	profile       string
	imdsEndpoint  string
	pollInterval  time.Duration
	nextStartTime time.Time
	logger        *zap.Logger
	consumer      consumer.Metrics
	wg            *sync.WaitGroup
	doneChan      chan bool
	client        client
	config        *Config
}

func newMetricReceiver(cfg *Config, logger *zap.Logger, consumer consumer.Metrics) (*metricReceiver, error) {
	client, err := newCloudWatchClient(cfg, logger)
	if err != nil {
		return nil, err
	}

	return &metricReceiver{
		region:        cfg.Region,
		profile:       cfg.Profile,
		imdsEndpoint:  cfg.IMDSEndpoint,
		pollInterval:  cfg.PollInterval,
		nextStartTime: time.Now().Add(-cfg.PollInterval),
		logger:        logger,
		wg:            &sync.WaitGroup{},
		consumer:      consumer,
		doneChan:      make(chan bool),
		client:        client,
		config:        cfg,
	}, nil
}

func (m *metricReceiver) Start(ctx context.Context, _ component.Host) error {
	m.logger.Debug("starting to poll for CloudWatch metrics")
	m.wg.Add(1)
	go m.startPolling(ctx)
	return nil
}

func (m *metricReceiver) Shutdown(_ context.Context) error {
	m.logger.Debug("shutting down awscloudwatchmetrics receiver")
	close(m.doneChan)
	m.wg.Wait()
	return nil
}

func (m *metricReceiver) startPolling(ctx context.Context) {
	defer m.wg.Done()

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
				m.logger.Error("Error polling metrics", zap.Error(err))
			}
		}
	}
}

func (m *metricReceiver) poll(ctx context.Context) error {
	endTime := time.Now()
	startTime := m.nextStartTime

	m.logger.Debug("Polling metrics",
		zap.Time("startTime", startTime),
		zap.Time("endTime", endTime))

	// Build metric data queries for all configured metrics
	var queries []cloudwatch.MetricDataQuery
	for _, metricConfig := range m.config.Metrics.Names {
		query := buildMetricDataQuery(
			metricConfig.Namespace,
			metricConfig.MetricName,
			metricConfig.Period,
			metricConfig.AwsAggregation,
			metricConfig.Dimensions,
		)
		queries = append(queries, query)
	}

	// Get metric data from CloudWatch
	input := &cloudwatch.GetMetricDataInput{
		StartTime:         aws.Time(startTime),
		EndTime:           aws.Time(endTime),
		MetricDataQueries: queries,
	}

	output, err := m.client.GetMetricData(ctx, input)
	if err != nil {
		return err
	}

	// Convert CloudWatch metrics to OpenTelemetry metrics
	metrics := m.convertToMetrics(output)
	if metrics.ResourceMetrics().Len() > 0 {
		if err := m.consumer.ConsumeMetrics(ctx, metrics); err != nil {
			m.logger.Error("Error consuming metrics", zap.Error(err))
			return err
		}
	}

	// Update next start time
	m.nextStartTime = endTime
	return nil
}

func (m *metricReceiver) convertToMetrics(output *cloudwatch.GetMetricDataOutput) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	if len(output.MetricDataResults) == 0 {
		return metrics
	}

	rm := metrics.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr("cloud.provider", "aws")
	resource.Attributes().PutStr("cloud.region", m.region)

	sm := rm.ScopeMetrics().AppendEmpty()
	scope := sm.Scope()
	scope.SetName("awscloudwatchmetricsreceiver")
	scope.SetVersion("1.0.0")

	for _, result := range output.MetricDataResults {
		if len(result.Values) == 0 {
			continue
		}

		metric := sm.Metrics().AppendEmpty()
		metric.SetName(*result.Label)
		metric.SetUnit(result.Unit.String())

		// Create a data point for each value
		dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(result.Timestamps[0].UnixNano()))
		dp.SetDoubleValue(result.Values[0])

		// Add metric attributes
		attributes := dp.Attributes()
		attributes.PutStr("namespace", *result.Id)
		if result.StatusCode != nil {
			attributes.PutStr("status_code", result.StatusCode.String())
		}
	}

	return metrics
}
