// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelector // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type (
	StartCallback = func(context.Context)
	StopCallback  = func()
)

// LeaderElection Interface allows the invoker to set the callback functions
// that would be invoked when the leader wins or loss the election.
type LeaderElection interface {
	extension.Extension
	SetCallBackFuncs(StartCallback, StopCallback)
}

type callBackFuncs struct {
	onStartLeading StartCallback
	onStopLeading  StopCallback
}

// leaderElectionExtension is the main struct implementing the extension's behavior.
type leaderElectionExtension struct {
	config        *Config
	client        kubernetes.Interface
	logger        *zap.Logger
	leaseHolderID string
	cancel        context.CancelFunc
	waitGroup     sync.WaitGroup

	callBackFuncs []callBackFuncs

	// ready is true after the collector signals that all extensions and pipelines
	// are initialized. Callbacks are only invoked after this is set.
	ready bool
	// isLeader is true when this instance holds the leader lease.
	isLeader bool
	// leaderCtx is the context passed to startedLeading, stored for use in Ready()
	// if leadership was acquired before the collector was ready.
	leaderCtx context.Context

	mu sync.Mutex
}

var _ extensioncapabilities.PipelineWatcher = (*leaderElectionExtension)(nil)

// SetCallBackFuncs set the functions that can be invoked when the leader wins or loss the election
func (lee *leaderElectionExtension) SetCallBackFuncs(onStartLeading StartCallback, onStopLeading StopCallback) {
	lee.mu.Lock()
	defer lee.mu.Unlock()

	lee.callBackFuncs = append(lee.callBackFuncs, callBackFuncs{
		onStartLeading: onStartLeading,
		onStopLeading:  onStopLeading,
	})

	// Only invoke immediately if both ready and leader. If leader but not ready,
	// the callback will be invoked when Ready() is called.
	if lee.ready && lee.isLeader {
		onStartLeading(lee.leaderCtx)
	}
}

// If the receiver sets a callback function then it would be invoked when the leader wins the election
func (lee *leaderElectionExtension) startedLeading(ctx context.Context) {
	lee.mu.Lock()
	defer lee.mu.Unlock()

	lee.isLeader = true
	lee.leaderCtx = ctx

	// Only invoke callbacks if the collector is ready. If not ready yet,
	// callbacks will be invoked when Ready() is called.
	if !lee.ready {
		return
	}

	for _, callback := range lee.callBackFuncs {
		callback.onStartLeading(ctx)
	}
}

// If the receiver sets a callback function then it would be invoked when the leader loss the election
func (lee *leaderElectionExtension) stoppedLeading() {
	// Have a read lock while stopping the receivers. This would make sure that if we have executed any onStartLeading callbacks
	// after becoming leader, we would execute the onStopLeading callbacks for them as well.
	lee.mu.Lock()
	defer lee.mu.Unlock()

	lee.isLeader = false
	if !lee.ready {
		return
	}

	for _, callback := range lee.callBackFuncs {
		callback.onStopLeading()
	}
}

// Start begins the extension's processing.
func (lee *leaderElectionExtension) Start(_ context.Context, _ component.Host) error {
	lee.logger.Info("Starting k8s leader elector with UUID", zap.String("UUID", lee.leaseHolderID))

	ctx := context.Background()
	ctx, lee.cancel = context.WithCancel(ctx)

	// Create the K8s leader elector
	leaderElector, err := newK8sLeaderElector(lee.config, lee.client, lee.startedLeading, lee.stoppedLeading, lee.leaseHolderID)
	if err != nil {
		lee.logger.Error("Failed to create k8s leader elector", zap.Error(err))
		return err
	}
	lee.waitGroup.Add(1)
	go func() {
		// Leader election loop stops if context is canceled or the leader elector loses the lease.
		// The loop allows continued participation in leader election, even if the lease is lost.
		defer lee.waitGroup.Done()
		for {
			leaderElector.Run(ctx)
			if ctx.Err() != nil {
				break
			}

			lee.logger.Info("Leader lease lost. Returning to standby mode...")
		}
	}()

	return nil
}

// Shutdown ends the extension's processing.
func (lee *leaderElectionExtension) Shutdown(context.Context) error {
	lee.logger.Info("Stopping k8s leader elector with UUID", zap.String("UUID", lee.leaseHolderID))
	if lee.cancel != nil {
		lee.cancel()
	}
	lee.waitGroup.Wait()
	return nil
}

// Ready is called by the collector when all extensions and pipelines are initialized.
// If leadership was acquired before this, the onStartLeading callbacks are invoked now.
func (lee *leaderElectionExtension) Ready() error {
	lee.mu.Lock()
	defer lee.mu.Unlock()

	lee.ready = true

	if lee.isLeader {
		for _, callback := range lee.callBackFuncs {
			callback.onStartLeading(lee.leaderCtx)
		}
	}
	return nil
}

// NotReady is called by the collector before shutdown.
func (*leaderElectionExtension) NotReady() error {
	return nil
}
