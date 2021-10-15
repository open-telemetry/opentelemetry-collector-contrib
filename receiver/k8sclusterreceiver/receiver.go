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
	"fmt"
	"time"

	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"k8s.io/client-go/kubernetes"
)

const (
	transport = "http"

	defaultInitialSyncTimeout = 10 * time.Minute
)

var _ component.MetricsReceiver = (*kubernetesReceiver)(nil)

type kubernetesReceiver struct {
	resourceWatcher *resourceWatcher

	config   *Config
	settings component.ReceiverCreateSettings
	consumer consumer.Metrics
	cancel   context.CancelFunc
	obsrecv  *obsreport.Receiver
}

func (kr *kubernetesReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, kr.cancel = context.WithCancel(ctx)

	exporters := host.GetExporters()
	if err := kr.resourceWatcher.setupMetadataExporters(
		exporters[config.MetricsDataType], kr.config.MetadataExporters); err != nil {
		return err
	}

	go func() {
		kr.settings.Logger.Info("Starting shared informers and wait for initial cache sync.")
		for _, informer := range kr.resourceWatcher.informerFactories {
			if informer == nil {
				continue
			}
			timedContextForInitialSync := kr.resourceWatcher.startWatchingResources(ctx, informer)

			// Wait till either the initial cache sync times out or until the cancel method
			// corresponding to this context is called.
			<-timedContextForInitialSync.Done()

			// If the context times out, set initialSyncTimedOut and report a fatal error. Currently
			// this timeout is 10 minutes, which appears to be long enough.
			if timedContextForInitialSync.Err() == context.DeadlineExceeded {
				kr.resourceWatcher.initialSyncTimedOut.Store(true)
				kr.settings.Logger.Error("Timed out waiting for initial cache sync.")
				host.ReportFatalError(fmt.Errorf("failed to start receiver: %v", kr.config.ID()))
				return
			}
		}

		kr.settings.Logger.Info("Completed syncing shared informer caches.")
		kr.resourceWatcher.initialSyncDone.Store(true)

		ticker := time.NewTicker(kr.config.CollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				kr.dispatchMetrics(ctx)
			case <-ctx.Done():
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

	c := kr.obsrecv.StartMetricsOp(ctx)

	numPoints := mds.DataPointCount()
	err := kr.consumer.ConsumeMetrics(c, mds)
	kr.obsrecv.EndMetricsOp(c, typeStr, numPoints, err)
}

// newReceiver creates the Kubernetes cluster receiver with the given configuration.
func newReceiver(
	set component.ReceiverCreateSettings, config *Config, consumer consumer.Metrics,
	client kubernetes.Interface, osQuotaClient quotaclientset.Interface) (component.MetricsReceiver, error) {
	resourceWatcher := newResourceWatcher(set.Logger, client, osQuotaClient, config.NodeConditionTypesToReport, defaultInitialSyncTimeout)

	return &kubernetesReceiver{
		resourceWatcher: resourceWatcher,
		settings:        set,
		config:          config,
		consumer:        consumer,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              transport,
			ReceiverCreateSettings: set,
		}),
	}, nil
}
