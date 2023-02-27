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
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
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
	GetMetricDataPagesWithContext(ctx aws.Context, input *cloudwatch.GetMetricDataInput, fn func(*cloudwatch.GetMetricDataOutput, bool) bool, opts ...request.Option) error
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
	Dimensions     []*namedRequestDimensions
}

type namedRequestDimensions struct {
	Name  string
	Value string
}

func (nr *namedRequest) request(st, et *time.Time, rqid string) *cloudwatch.GetMetricDataOutput {
	return nil
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
}

func (m *metricReceiver) pollForMetrics(ctx context.Context, r namedRequest, startTime time.Time, endTime time.Time) error {

}
