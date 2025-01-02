package leaderelector

import (
	"context"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type LeaderElection interface {
	extension.Extension
	SetCallBackFuncs(func(ctx context.Context), func())
}

// Sets the callBack functions that can be invoked when the leader wins or loss the election
func (lee *leaderElectionExtension) SetCallBackFuncs(onStartLeading func(context.Context), onStopLeading func()) {
	fmt.Printf("Setting callback functions!!!\n")
	lee.onStartedLeading = onStartLeading
	lee.onStoppedLeading = onStopLeading
}

// leaderElectionExtension is the main struct implementing the extension's behavior.
type leaderElectionExtension struct {
	config *Config
	logger *zap.Logger

	onStartedLeading func(context.Context)
	onStoppedLeading func()
	iamLeader        bool
	cancel           context.CancelFunc
	client           kubernetes.Interface
}

// If the receiver sets a callback function then it would be invoked when the leader wins the election
// additionally set iamLeader to true
func (lee *leaderElectionExtension) startedLeading(ctx context.Context) {
	lee.iamLeader = true
	if lee.onStartedLeading != nil {
		lee.onStartedLeading(ctx)
	}
}

// If the receiver sets a callback function then it would be invoked when the leader loss the election
// additionally set iamLeader to false
func (lee *leaderElectionExtension) stoppedLeading() {
	lee.iamLeader = false
	if lee.onStoppedLeading != nil {
		lee.onStoppedLeading()
	}
}

func (lee *leaderElectionExtension) AmILeader() bool {
	return lee.iamLeader
}

// Start begins the extension's processing.
func (lee *leaderElectionExtension) Start(_ context.Context, host component.Host) error {
	// Implement your start logic here
	lee.logger.Info("I am starting Leader Election!!")

	ctx := context.Background()
	ctx, lee.cancel = context.WithCancel(ctx)

	// Create the leader elector
	leaderElector, err := NewLeaderElector(lee.config, lee.client, lee.startedLeading, lee.stoppedLeading, "testID")
	if err != nil {
		lee.logger.Error("Failed to create leader elector", zap.Error(err))
		return err
	}

	go func() {
		for {
			leaderElector.Run(ctx)
		}
	}()

	return nil
}

// Shutdown cleans up the extension when stopping.
func (lee *leaderElectionExtension) Shutdown(ctx context.Context) error {
	// Implement your shutdown logic here
	lee.logger.Info("I a stopping!!")
	return nil
}
