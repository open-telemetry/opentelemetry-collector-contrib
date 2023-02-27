// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type metricReceiver struct {
	region        string
	profile       string
	imdsEndpoint  string
	pollInterval  time.Duration
	nextStartTime time.Time
	logger        *zap.Logger
	client        *cloudwatch.Client
	namedRequest  []namedRequest
	mc            *MetricsConfig
	consumer      consumer.Metrics
	wg            *sync.WaitGroup
	doneChan      chan bool
}

type client interface {
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

type metricsRequest interface {
	request(st, et *time.Time, rqid string) *cloudwatch.GetMetricDataOutput
	name() string
}

type namedRequest struct {
	Namespace      string
	MetricName     string
	Period         time.Duration
	AwsAggregation string
	Dimensions     []types.Dimension
}

func (nr *namedRequest) request(st, et *time.Time, rqid string) *cloudwatch.GetMetricDataInput {
	getMetricInput := &cloudwatch.GetMetricDataInput{
		EndTime:   et,
		StartTime: st,
		MetricDataQueries: []types.MetricDataQuery{
			types.MetricDataQuery{
				Id: aws.String(rqid),
				MetricStat: &types.MetricStat{
					Metric: &types.Metric{
						Namespace:  &nr.Namespace,
						MetricName: &nr.MetricName,
						Dimensions: nr.Dimensions,
					},
					Period: aws.Int32(int32(math.Abs(nr.Period.Seconds()))),
					Stat:   &nr.AwsAggregation,
				},
				ReturnData: aws.Bool(true),
			},
		},
	}

	return getMetricInput
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

func (m *metricReceiver) Start(ctx context.Context, host component.Host) error {
	m.logger.Debug("Starting to poll for CloudWatch metrics")
	m.wg.Add(1)
	go m.poll(ctx)
	return nil
}

func (m *metricReceiver) Shutdown(ctx context.Context) error {
	m.logger.Debug("Shutting down awscloudwatchmetrics receiver")
	close(m.doneChan)
	m.wg.Wait()
	return nil
}

func (m *metricReceiver) startPolling(ctx context.Context) {
	defer m.wg.Done()

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
	for _, r := range m.namedRequest {
		if err := m.pollForMetrics(ctx, r, startTime, endTime); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	m.nextStartTime = endTime
	return errs
}

func (m *metricReceiver) pollForMetrics(ctx context.Context, r namedRequest, startTime time.Time, endTime time.Time) error {
	select {
	case _, ok := <-m.doneChan:
		if !ok {
			return nil
		}
	default:
		filter := r.request(&startTime, &endTime, "test")
		paginator := cloudwatch.NewGetMetricDataPaginator(m.client, filter)
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				m.logger.Error("unable to retrive metric data from cloudwatch", zap.String("metric name", r.MetricName), zap.Error(err))
				break
			}
			observedTime := pcommon.NewTimestampFromTime(time.Now())
			metrics := m.parseMetrics(observedTime, r.Namespace, r.MetricName, output)
			if metrics.MetricCount() > 0 {
				if err = m.consumer.ConsumeMetrics(ctx, metrics); err != nil {
					m.logger.Error("unable to consume logs", zap.Error(err))
					break
				}
			}
		}
	}
	return nil
}

func (m *metricReceiver) parseMetrics(observedTime pcommon.Timestamp, namespace, metricName string, output *cloudwatch.GetMetricDataOutput) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for _, metric := range output.MetricDataResults {
		if len(metric.Timestamps) < 1 {
			m.logger.Error("no timestamps received from cloudwatch")
			return metrics
		}
	}
	return metrics
}
