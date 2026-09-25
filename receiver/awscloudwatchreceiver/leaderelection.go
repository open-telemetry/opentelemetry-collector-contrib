// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"
)

func getLeaderElector(host component.Host, id component.ID) (k8sleaderelector.LeaderElection, error) {
	ext := host.GetExtensions()[id]
	if ext == nil {
		return nil, fmt.Errorf("unknown k8s leader elector %q", id)
	}
	elector, ok := ext.(k8sleaderelector.LeaderElection)
	if !ok {
		return nil, fmt.Errorf("the extension %T does not implement k8sleaderelector.LeaderElection", ext)
	}
	return elector, nil
}

// leaderElectedMetrics gates a metrics controller on Kubernetes leadership. The controller is
// rebuilt for every term, since a collector component is not required to be restartable.
type leaderElectedMetrics struct {
	electorID     component.ID
	logger        *zap.Logger
	newController func() (receiver.Metrics, error)

	mu      sync.Mutex
	host    component.Host
	current receiver.Metrics
}

var _ receiver.Metrics = (*leaderElectedMetrics)(nil)

func newLeaderElectedMetrics(electorID component.ID, logger *zap.Logger, newController func() (receiver.Metrics, error)) *leaderElectedMetrics {
	return &leaderElectedMetrics{
		electorID:     electorID,
		logger:        logger,
		newController: newController,
	}
}

func (l *leaderElectedMetrics) Start(_ context.Context, host component.Host) error {
	elector, err := getLeaderElector(host, l.electorID)
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.host = host
	l.mu.Unlock()

	// The extension invokes the start callback inline when this instance is already the leader.
	elector.SetCallBackFuncs(l.startLeading, l.stopLeading)
	return nil
}

func (l *leaderElectedMetrics) startLeading(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current != nil {
		return
	}

	controller, err := l.newController()
	if err != nil {
		l.logger.Error("failed to build metrics controller after acquiring leadership", zap.Error(err))
		return
	}
	if err := controller.Start(ctx, l.host); err != nil {
		l.logger.Error("failed to start metrics controller after acquiring leadership", zap.Error(err))
		return
	}
	l.current = controller
}

func (l *leaderElectedMetrics) stopLeading() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.shutdownCurrent(context.Background())
}

func (l *leaderElectedMetrics) Shutdown(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.shutdownCurrent(ctx)
	return nil
}

func (l *leaderElectedMetrics) shutdownCurrent(ctx context.Context) {
	if l.current == nil {
		return
	}
	if err := l.current.Shutdown(ctx); err != nil {
		l.logger.Error("failed to shut down metrics controller", zap.Error(err))
	}
	l.current = nil
}
