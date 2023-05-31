// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"
	"fmt"
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
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type metricReceiver struct {
	region        string
	profile       string
	imdsEndpoint  string
	pollInterval  time.Duration
	nextStartTime time.Time
	logger        *zap.Logger
	client        client
	namedRequests []namedRequest
	consumer      consumer.Metrics
	wg            *sync.WaitGroup
	doneChan      chan bool
}

type namedRequest struct {
	Namespace      string
	MetricName     string
	Period         time.Duration
	AwsAggregation string
	Dimensions     []types.Dimension
	Values         *types.MetricDataResult
	observedTime   *pcommon.Timestamp
}

type client interface {
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

const (
	maxNumberOfElements = 500
)

func buildGetMetricDataQueries(metric *namedRequest) types.MetricDataQuery {
	mdq := types.MetricDataQuery{
		Id:         aws.String("m1"),
		ReturnData: aws.Bool(true),
	}
	mdq.MetricStat = &types.MetricStat{
		Metric: &types.Metric{
			Namespace:  aws.String(metric.Namespace),
			MetricName: aws.String(metric.MetricName),
			Dimensions: metric.Dimensions,
		},
		Period: aws.Int32(int32(metric.Period / time.Second)),
		Stat:   aws.String(metric.AwsAggregation),
	}
	return mdq
}

func chunkSlice(namedRequestMetrics []namedRequest, maxSize int) [][]namedRequest {
	var slicedMetrics [][]namedRequest
	for i := 0; i < len(namedRequestMetrics); i += maxSize {
		end := i + maxSize

		if end > len(namedRequestMetrics) {
			end = len(namedRequestMetrics)
		}
		slicedMetrics = append(slicedMetrics, namedRequestMetrics[i:end])
	}
	return slicedMetrics
}

// divide up into slices of 500, then execute
// Split metricNamedRequest slices into small slicer no longer than 500 elements
// GetMetricData only allows 500 elements in a slice, otherwise we'll get validation error
// Avoids making a network call for each metric configured
func (m *metricReceiver) request(st, et *time.Time) []cloudwatch.GetMetricDataInput {
	metricDataInput := make([]cloudwatch.GetMetricDataInput, len(m.namedRequests))

	chunks := chunkSlice(m.namedRequests, maxNumberOfElements)
	for idx, chunk := range chunks {
		for ydx := range chunk {
			metricDataInput[idx].StartTime = st
			metricDataInput[idx].EndTime = et
			metricDataInput[idx].MetricDataQueries =
				append(metricDataInput[idx].MetricDataQueries, buildGetMetricDataQueries(&chunk[ydx]))
		}
	}
	return metricDataInput
}

func newMetricReceiver(cfg *Config, logger *zap.Logger, consumer consumer.Metrics) *metricReceiver {
	var requests []namedRequest
	for idx, nc := range cfg.Metrics.Names {
		logger.Debug("metric name ", zap.String("", nc.MetricName))
		var dimensions []types.Dimension
		for ydx := range nc.Dimensions {
			dimensions = append(dimensions,
				types.Dimension{Name: &cfg.Metrics.Names[idx].Dimensions[ydx].Name, Value: &cfg.Metrics.Names[idx].Dimensions[ydx].Value})
		}

		requests = append(requests, namedRequest{
			Namespace:      nc.Namespace,
			MetricName:     nc.MetricName,
			Period:         nc.Period,
			AwsAggregation: nc.AwsAggregation,
			Dimensions:     dimensions,
		})
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
		namedRequests: requests,
		doneChan:      make(chan bool),
	}
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

	err := m.configureAWSClient()
	if err != nil {
		m.logger.Error("unable to establish connection to cloudwatch", zap.Error(err))
	}

	t := time.NewTicker(m.pollInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.doneChan:
			return
		case <-t.C:
			err := m.poll(ctx)
			if err != nil {
				m.logger.Error("there was an error during the poll", zap.Error(err))
			}
		}
	}
}

func (m *metricReceiver) poll(ctx context.Context) error {
	var errs error
	startTime := m.nextStartTime
	endTime := time.Now()
	if err := m.pollForMetrics(ctx, m.namedRequests, startTime, endTime); err != nil {
		errs = multierr.Append(errs, err)
	}
	m.nextStartTime = endTime
	return errs
}

func (m *metricReceiver) pollForMetrics(ctx context.Context, r []namedRequest, startTime time.Time, endTime time.Time) error {
	select {
	case _, ok := <-m.doneChan:
		if !ok {
			return nil
		}
	default:
		filters := m.request(&startTime, &endTime)
		var ret []types.MetricDataResult
		for idx := range filters {
			paginator := cloudwatch.NewGetMetricDataPaginator(m.client, &filters[idx])
			for paginator.HasMorePages() {
				output, err := paginator.NextPage(ctx)
				if err != nil {
					m.logger.Error("unable to retrieve metric data from cloudwatch", zap.Error(err))
					break
				}
				ret = append(ret, output.MetricDataResults...)
			}
		}
		observedTime := pcommon.NewTimestampFromTime(time.Now())
		for i := range r {
			r[i].Values = &ret[i]
			r[i].observedTime = &observedTime
		}
		metrics := m.parseMetrics()
		if metrics.MetricCount() > 0 {
			if err := m.consumer.ConsumeMetrics(ctx, metrics); err != nil {
				m.logger.Error("unable to consume logs", zap.Error(err))
				return err
			}
		}
	}
	return nil
}

func (m *metricReceiver) parseMetrics() pmetric.Metrics {
	pdm := pmetric.NewMetrics()
	rms := pdm.ResourceMetrics()
	rm := rms.AppendEmpty()
	rm.Resource().Attributes()
	ilms := rm.ScopeMetrics()
	ilm := ilms.AppendEmpty()
	ms := ilm.Metrics()
	ms.EnsureCapacity(len(m.namedRequests))
	for i := 0; i < len(m.namedRequests); i++ {
		fillDataPoints(ms.AppendEmpty(), &m.namedRequests[i])
	}
	return pdm
}

func fillDataPoints(pdm pmetric.Metric, nr *namedRequest) pmetric.Metric {
	atts := make(map[string]interface{})
	pdm.SetName(fmt.Sprintf("%s.%s", nr.Namespace, nr.MetricName))
	dps := pdm.SetEmptyGauge().DataPoints()

	for _, dim := range nr.Dimensions {
		atts[*dim.Name] = dim.Value
	}

	for idx, value := range nr.Values.Values {
		timestamp := nr.Values.Timestamps[idx]
		dp := dps.AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetStartTimestamp(*nr.observedTime)
		dp.SetDoubleValue(value)
		for _, dim := range nr.Dimensions {
			dp.Attributes().PutStr(*dim.Name, *dim.Value)
		}
	}
	return pdm
}

func (m *metricReceiver) configureAWSClient() error {
	if m.client != nil {
		return nil
	}
	// if "", helper functions (withXXX) ignores parameter
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(m.region), config.WithEC2IMDSEndpoint(m.imdsEndpoint), config.WithSharedConfigProfile(m.profile))
	m.client = cloudwatch.NewFromConfig(cfg)
	return err
}
