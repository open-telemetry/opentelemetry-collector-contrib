// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelector // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"

import (
	"context"
	"sync"
	"sync/atomic"

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

type callBackFuncs struct {
	onStartLeading StartCallback
	onStopLeading  StopCallback
}

// SetCallBackFuncs set the functions that can be invoked when the leader wins or loss the election
func (lee *leaderElectionExtension) SetCallBackFuncs(onStartLeading StartCallback, onStopLeading StopCallback) {
	// If the extension has already started, and it has become the leader, then channel is already created so we can push the callbacks to it.
	callBack := callBackFuncs{
		onStartLeading: onStartLeading,
		onStopLeading:  onStopLeading,
	}
	if lee.callBackChan != nil && lee.iAmLeader.Load() {
		lee.callBackChan <- callBack
	}

	// Append the callbacks to the lists for following cases
	// 1. When the leader election is lost, and in case its gained again then the callbacks should be invoked again.
	// 2. When the extension has started, but it is not the leader, then the callbacks should be stored.
	lee.mu.Lock()
	defer lee.mu.Unlock()
	lee.callBackFuncs = append(lee.callBackFuncs, callBack)
}

// leaderElectionExtension is the main struct implementing the extension's behavior.
type leaderElectionExtension struct {
	config        *Config
	client        kubernetes.Interface
	logger        *zap.Logger
	leaseHolderID string
	cancel        context.CancelFunc
	waitGroup     sync.WaitGroup

	callBackChan  chan callBackFuncs
	iAmLeaderChan chan bool
	callBackFuncs []callBackFuncs

	iAmLeader atomic.Bool

	mu sync.Mutex
}

// If the receiver sets a callback function then it would be invoked when the leader wins the election
func (lee *leaderElectionExtension) startedLeading(ctx context.Context) {
	// Create a channel for receivers which have registered the callback after the extension has become the leader. In such case the callback is pushed to
	// channel and executed immediately.
	lee.callBackChan = make(chan callBackFuncs, 1)
	lee.iAmLeaderChan = make(chan bool)

	lee.iAmLeader.Store(true)
	lee.iAmLeaderChan <- lee.iAmLeader.Load()

	//for _, callback := range lee.callBackFuncs {
	//	callback.onStartLeading(ctx)
	//}
	//
	//go func() {
	//	for {
	//		select {
	//		case <-ctx.Done():
	//			return
	//		case callback, ok := <-lee.callBackChan:
	//			if !ok {
	//				return
	//			}
	//			if callback.onStartLeading != nil {
	//				callback.onStartLeading(ctx)
	//			}
	//		}
	//	}
	//}()
}

// If the receiver sets a callback function then it would be invoked when the leader loss the election
func (lee *leaderElectionExtension) stoppedLeading() {
	// We have lost the leader election, so we close the channel to stop receiving callbacks.
	close(lee.callBackChan)
	lee.iAmLeader.Store(false)

	// make sure for all the callbacks that were registered before the leader election was lost, we invoke the stopLeading callback.
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
		// if we have no value pushed to iAmLeaderChan, then we never became leader and it would block forever.

		for {
			select {
			case iAmLeader := <-lee.iAmLeaderChan:
				if !iAmLeader {
					// stop the callbacks if we are not the leader
				}
				// run all the callbacks
				callback := <-lee.callBackChan
				found := false
				for _, cb := range lee.callBackFuncs {
					if cb.onStartLeading != nil && cb.onStartLeading == callback.onStartLeading {
						found = true
						break
					}
				}

				// execute the list as well But there can be duplicacy as callback is pushed to channel and also stored in the list.
				for _, cb := range lee.callBackFuncs {
					if cb.onStartLeading != nil {
						cb.onStartLeading(ctx)
					}
				}

				// fetch from channel

			}
		}
	}()

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
