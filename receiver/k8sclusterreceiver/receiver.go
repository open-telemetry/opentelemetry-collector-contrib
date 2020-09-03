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
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

const (
	transport = "http"
)

var _ component.MetricsReceiver = (*kubernetesReceiver)(nil)

type kubernetesReceiver struct {
	resourceWatcher *resourceWatcher

	config   *Config
	logger   *zap.Logger
	consumer consumer.MetricsConsumer
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
				kr.dispatchMetrics(c)
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

func (kr *kubernetesReceiver) dispatchMetrics(ctx context.Context) {
	now := time.Now()
	mds := kr.resourceWatcher.dataCollector.CollectMetricData(now)
	resourceMetrics := internaldata.OCSliceToMetrics(mds)

	c := obsreport.StartMetricsReceiveOp(ctx, typeStr, transport)

	numTimeseries, numPoints := resourceMetrics.MetricAndDataPointCount()

	err := kr.consumer.ConsumeMetrics(c, resourceMetrics)
	obsreport.EndMetricsReceiveOp(c, typeStr, numPoints, numTimeseries, err)
}

// newReceiver creates the Kubernetes cluster receiver with the given configuration.
func newReceiver(
	logger *zap.Logger, config *Config, consumer consumer.MetricsConsumer,
	client kubernetes.Interface) (component.MetricsReceiver, error) {
	resourceWatcher := newResourceWatcher(logger, client, config.NodeConditionTypesToReport)

	return &kubernetesReceiver{
		resourceWatcher: resourceWatcher,
		logger:          logger,
		config:          config,
		consumer:        consumer,
	}, nil
}
