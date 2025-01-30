// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func NewLeaderElector(
	cfg *Config,
	client kubernetes.Interface,
	onStartedLeading func(context.Context),
	onStoppedLeading func(),
	identity string,
) (*leaderelection.LeaderElector, error) {
	resourceLock, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		cfg.LeaseNamespace,
		cfg.LeaseName,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: identity,
		})
	if err != nil {
		return nil, err
	}

	leConfig := leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: cfg.LeaseDuration,
		RenewDeadline: cfg.RenewDuration,
		RetryPeriod:   cfg.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onStartedLeading,
			OnStoppedLeading: onStoppedLeading,
		},
	}

	return leaderelection.NewLeaderElector(leConfig)
}
