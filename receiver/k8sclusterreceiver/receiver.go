// Copyright 2020, OpenTelemetry Authors
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

package k8sclusterreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
)

const (
	transport = "http"
)

var _ component.MetricsReceiver = (*kubernetesReceiver)(nil)

type kubernetesReceiver struct {
	resourceWatcher *resourceWatcher

	config   *Config
	logger   *zap.Logger
	consumer consumer.MetricsConsumerOld
	cancel   context.CancelFunc
}

func (kr *kubernetesReceiver) Start(ctx context.Context, host component.Host) error {
	var c context.Context
	c, kr.cancel = context.WithCancel(obsreport.ReceiverContext(ctx, typeStr, transport, kr.config.Name()))

	exporters := host.GetExporters()
	if err := kr.resourceWatcher.setupMetadataExporters(
		exporters[configmodels.MetricsDataType], kr.config.MetadataExporters); err != nil {
		return err
	}

	go func() {
		kr.resourceWatcher.startWatchingResources(c.Done())

		ticker := time.NewTicker(kr.config.CollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				kr.dispatchMetricData(c)
			case <-c.Done():
				return
			}
		}
	}()

	return nil
}

func (kr *kubernetesReceiver) Shutdown(context.Context) error {
	kr.cancel()
	return nil
}

func (kr *kubernetesReceiver) dispatchMetricData(ctx context.Context) {
	for _, m := range kr.resourceWatcher.dataCollector.CollectMetricData() {
		c := obsreport.StartMetricsReceiveOp(ctx, typeStr, transport)

		numTimeseries, numPoints := obsreport.CountMetricPoints(m)

		err := kr.consumer.ConsumeMetricsData(c, m)
		obsreport.EndMetricsReceiveOp(c, typeStr, numPoints, numTimeseries, err)
	}
}

// newReceiver creates the Kubernetes cluster receiver with the given configuration.
func newReceiver(
	logger *zap.Logger,
	config *Config,
	consumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {

	client, err := k8sconfig.MakeClient(config.APIConfig)
	if err != nil {
		return nil, err
	}

	resourceWatcher, err := newResourceWatcher(logger, config, client)
	if err != nil {
		return nil, err
	}

	r := &kubernetesReceiver{
		resourceWatcher: resourceWatcher,
		logger:          logger,
		config:          config,
		consumer:        consumer,
	}

	return r, nil
}
