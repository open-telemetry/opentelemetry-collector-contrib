// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/collection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	transport = "http"

	defaultInitialSyncTimeout = 10 * time.Minute
)

var _ receiver.Metrics = (*kubernetesReceiver)(nil)

type kubernetesReceiver struct {
	dataCollector   *collection.DataCollector
	resourceWatcher *resourceWatcher

	config          *Config
	settings        receiver.CreateSettings
	metricsConsumer consumer.Metrics
	cancel          context.CancelFunc
	obsrecv         *receiverhelper.ObsReport
}

func (kr *kubernetesReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, kr.cancel = context.WithCancel(ctx)

	if err := kr.resourceWatcher.initialize(); err != nil {
		return err
	}

	exporters := host.GetExporters() //nolint:staticcheck
	if err := kr.resourceWatcher.setupMetadataExporters(
		exporters[component.DataTypeMetrics], kr.config.MetadataExporters); err != nil {
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
			if errors.Is(timedContextForInitialSync.Err(), context.DeadlineExceeded) {
				kr.resourceWatcher.initialSyncTimedOut.Store(true)
				kr.settings.Logger.Error("Timed out waiting for initial cache sync.")
				host.ReportFatalError(errors.New("failed to start receiver"))
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
	if kr.cancel == nil {
		return nil
	}
	kr.cancel()
	return nil
}

func (kr *kubernetesReceiver) dispatchMetrics(ctx context.Context) {
	if kr.metricsConsumer == nil {
		// Metric collection is not enabled.
		return
	}

	mds := kr.dataCollector.CollectMetricData(time.Now())

	c := kr.obsrecv.StartMetricsOp(ctx)

	numPoints := mds.DataPointCount()
	err := kr.metricsConsumer.ConsumeMetrics(c, mds)
	kr.obsrecv.EndMetricsOp(c, metadata.Type, numPoints, err)
}

// newMetricsReceiver creates the Kubernetes cluster receiver with the given configuration.
func newMetricsReceiver(
	ctx context.Context, set receiver.CreateSettings, cfg component.Config, consumer consumer.Metrics,
) (receiver.Metrics, error) {
	var err error
	r := receivers.GetOrAdd(
		cfg, func() component.Component {
			var rcv component.Component
			rcv, err = newReceiver(ctx, set, cfg)
			return rcv
		},
	)
	if err != nil {
		return nil, err
	}
	r.Unwrap().(*kubernetesReceiver).metricsConsumer = consumer
	return r, nil
}

// newMetricsReceiver creates the Kubernetes cluster receiver with the given configuration.
func newLogsReceiver(
	ctx context.Context, set receiver.CreateSettings, cfg component.Config, consumer consumer.Logs,
) (receiver.Logs, error) {
	var err error
	r := receivers.GetOrAdd(
		cfg, func() component.Component {
			var rcv component.Component
			rcv, err = newReceiver(ctx, set, cfg)
			return rcv
		},
	)
	if err != nil {
		return nil, err
	}
	r.Unwrap().(*kubernetesReceiver).resourceWatcher.entityLogConsumer = consumer
	return r, nil
}

// newMetricsReceiver creates the Kubernetes cluster receiver with the given configuration.
func newReceiver(_ context.Context, set receiver.CreateSettings, cfg component.Config) (component.Component, error) {
	rCfg := cfg.(*Config)
	obsrecv, err := receiverhelper.NewObsReport(
		receiverhelper.ObsReportSettings{
			ReceiverID:             set.ID,
			Transport:              transport,
			ReceiverCreateSettings: set,
		},
	)
	if err != nil {
		return nil, err
	}
	ms := metadata.NewStore()
	return &kubernetesReceiver{
		dataCollector: collection.NewDataCollector(set, ms, rCfg.MetricsBuilderConfig,
			rCfg.NodeConditionTypesToReport, rCfg.AllocatableTypesToReport),
		resourceWatcher: newResourceWatcher(set, rCfg, ms),
		settings:        set,
		config:          rCfg,
		obsrecv:         obsrecv,
	}, nil
}
