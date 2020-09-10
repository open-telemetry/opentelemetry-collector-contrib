// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetricsreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/awsecscontainermetrics"
)

var _ component.MetricsReceiver = (*awsEcsContainerMetricsReceiver)(nil)

// awsEcsContainerMetricsReceiver implements the component.MetricsReceiver for aws ecs container metrics.
type awsEcsContainerMetricsReceiver struct {
	logger       *zap.Logger
	nextConsumer consumer.MetricsConsumer
	config       *Config
	cancel       context.CancelFunc
}

// newAwsEcsContainerMetricsReceiver creates the aws ecs container metrics receiver with the given parameters.
func newAwsEcsContainerMetricsReceiver(
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.MetricsConsumer) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	r := &awsEcsContainerMetricsReceiver{
		logger:       logger,
		nextConsumer: nextConsumer,
		config:       config,
	}
	return r, nil
}

// Start begins collecting metrics from Amazon ECS task metadata endpoint.
func (aecmr *awsEcsContainerMetricsReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, aecmr.cancel = context.WithCancel(obsreport.ReceiverContext(ctx, typeStr, "http", aecmr.config.Name()))
	go func() {
		ticker := time.NewTicker(aecmr.config.CollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				aecmr.collectDataFromEndpoint(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// Shutdown stops the awsecscontainermetricsreceiver receiver.
func (aecmr *awsEcsContainerMetricsReceiver) Shutdown(context.Context) error {
	aecmr.cancel()
	return nil
}

// collectDataFromEndpoint collects container stats from Amazon ECS Task Metadata Endpoint
// TODO: Replace with acutal logic.
func (aecmr *awsEcsContainerMetricsReceiver) collectDataFromEndpoint(ctx context.Context) error {
	md := awsecscontainermetrics.GenerateDummyMetrics()

	err := aecmr.nextConsumer.ConsumeMetrics(ctx, internaldata.OCToMetrics(md))
	return err
}
