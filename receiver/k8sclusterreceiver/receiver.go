// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	transport = "http"

	defaultInitialSyncTimeout = 10 * time.Minute
)

var _ receiver.Metrics = (*kubernetesReceiver)(nil)

type kubernetesReceiver struct {
	resourceWatcher *resourceWatcher

	config   *Config
	settings receiver.CreateSettings
	consumer consumer.Metrics
	cancel   context.CancelFunc
	obsrecv  *obsreport.Receiver
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
	now := time.Now()
	mds := kr.resourceWatcher.dataCollector.CollectMetricData(now)

	c := kr.obsrecv.StartMetricsOp(ctx)

	numPoints := mds.DataPointCount()
	err := kr.consumer.ConsumeMetrics(c, mds)
	kr.obsrecv.EndMetricsOp(c, metadata.Type, numPoints, err)
}

// newReceiver creates the Kubernetes cluster receiver with the given configuration.
func newReceiver(_ context.Context, set receiver.CreateSettings, cfg component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	rCfg := cfg.(*Config)

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             set.ID,
		Transport:              transport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}
	return &kubernetesReceiver{
		resourceWatcher: newResourceWatcher(set.Logger, rCfg),
		settings:        set,
		config:          rCfg,
		consumer:        consumer,
		obsrecv:         obsrecv,
	}, nil
}
