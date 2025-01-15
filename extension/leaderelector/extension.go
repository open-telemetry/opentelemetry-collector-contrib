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

// Start begins the extension's processing.
func (lee *leaderElectionExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown ends the extension's processing.
func (lee *leaderElectionExtension) Shutdown(context.Context) error {
	return nil
}
