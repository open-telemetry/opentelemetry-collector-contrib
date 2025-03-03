// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelector // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
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

// SetCallBackFuncs set the functions that can be invoked when the leader wins or loss the election
func (lee *leaderElectionExtension) SetCallBackFuncs(onStartLeading StartCallback, onStopLeading StopCallback) {
	lee.onStartedLeading = append(lee.onStartedLeading, onStartLeading)
	lee.onStoppedLeading = append(lee.onStoppedLeading, onStopLeading)
}

// leaderElectionExtension is the main struct implementing the extension's behavior.
type leaderElectionExtension struct {
	config        *Config
	client        kubernetes.Interface
	logger        *zap.Logger
	leaseHolderID string
	cancel        context.CancelFunc
	waitGroup     sync.WaitGroup

	onStartedLeading []StartCallback
	onStoppedLeading []StopCallback
}

// If the receiver sets a callback function then it would be invoked when the leader wins the election
func (lee *leaderElectionExtension) startedLeading(ctx context.Context) {
	for _, callback := range lee.onStartedLeading {
		callback(ctx)
	}
}

// If the receiver sets a callback function then it would be invoked when the leader loss the election
func (lee *leaderElectionExtension) stoppedLeading() {
	for _, callback := range lee.onStoppedLeading {
		callback()
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
