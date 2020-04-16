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

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	c, kr.cancel = context.WithCancel(ctx)

	go func() {
		kr.resourceWatcher.startWatchingResources(c.Done())

		ticker := time.NewTicker(kr.config.CollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				kr.dispatchMetricData(ctx)
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
		kr.consumer.ConsumeMetricsData(ctx, m)
	}
}

// newReceiver creates the Kubernetes cluster receiver with the given configuration.
func newReceiver(
	logger *zap.Logger,
	config *Config,
	consumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {

	k8sConfig, err := rest.InClusterConfig()

	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, err
	}
	resourceWatcher, err := newResourceWatcher(logger, config, client, false)

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
