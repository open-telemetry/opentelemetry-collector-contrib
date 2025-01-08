// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"

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
	cancel        context.CancelFunc
	client        kubernetes.Interface
	logger        *zap.Logger
	leaseHolderId string

	onStartedLeading []StartCallback
	onStoppedLeading []StopCallback
}

// If the receiver sets a callback function then it would be invoked when the leader wins the election
// additionally set iamLeader to true
func (lee *leaderElectionExtension) startedLeading(ctx context.Context) {
	for _, callback := range lee.onStartedLeading {
		callback(ctx)
	}
}

// If the receiver sets a callback function then it would be invoked when the leader loss the election
// additionally set iamLeader to false
func (lee *leaderElectionExtension) stoppedLeading() {
	for _, callback := range lee.onStoppedLeading {
		callback()
	}
}

// Start begins the extension's processing.
func (lee *leaderElectionExtension) Start(_ context.Context, host component.Host) error {
	lee.logger.Info("Starting Leader Elector")

	ctx := context.Background()
	ctx, lee.cancel = context.WithCancel(ctx)

	// Create the leader elector
	leaderElector, err := NewLeaderElector(lee.config, lee.client, lee.startedLeading, lee.stoppedLeading, lee.leaseHolderId)
	if err != nil {
		lee.logger.Error("Failed to create leader elector", zap.Error(err))
		return err
	}

	go func() {
		// Leader election loop stops if context is canceled or the leader elector loses the lease.
		// The loop allows continued participation in leader election, even if the lease is lost.
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

// Shutdown cleans up the extension when stopping.
func (lee *leaderElectionExtension) Shutdown(ctx context.Context) error {
	lee.logger.Info("Stopping Leader Elector")
	if lee.cancel != nil {
		lee.cancel()
	}
	return nil
}
