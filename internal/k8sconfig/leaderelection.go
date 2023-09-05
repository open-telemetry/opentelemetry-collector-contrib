// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// LeaderElectionConfig is used to enable leader election
type LeaderElectionConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// LockName determines the name of the resource that leader election will use for holding the leader lock.
	LockName      string        `mapstructure:"lock_name"`
	Namespace     string        `mapstructure:"namespace"`
	LeaseDuration time.Duration `mapstructure:"lease_duration"`
	RenewDeadline time.Duration `mapstructure:"renew_deadline"`
	RetryPeriod   time.Duration `mapstructure:"retry_period"`
}

const (
	inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultLeaseDuration   = 15 * time.Second
	defaultRenewDeadline   = 10 * time.Second
	defaultRetryPeriod     = 2 * time.Second
)

func NewDefaultLeaderElectionConfig() LeaderElectionConfig {
	return LeaderElectionConfig{
		Enabled:       false,
		LeaseDuration: defaultLeaseDuration,
		RenewDeadline: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
	}
}

// NewResourceLock creates a new leases resource lock for use in a leader election loop
func newResourceLock(client kubernetes.Interface, leaderElectionNamespace, lockName string) (resourcelock.Interface, error) {
	var err error
	if leaderElectionNamespace == "" {
		leaderElectionNamespace, err = getInClusterNamespace()
		if err != nil {
			return nil, fmt.Errorf("unable to find leader election namespace: %w", err)
		}
	}

	// Leader id, needs to be unique, use pod name in kubernetes case.
	id, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return resourcelock.New(resourcelock.LeasesResourceLock,
		leaderElectionNamespace, lockName,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		})
}

func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	_, err := os.Stat(inClusterNamespacePath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, unable to get namespace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

// NewLeaderElector return  a leader elector object using client-go
func NewLeaderElector(cfg LeaderElectionConfig, client kubernetes.Interface, startFunc func(context.Context), stopFunc func()) (*leaderelection.LeaderElector, error) {
	resourceLock, err := newResourceLock(client, cfg.Namespace, cfg.LockName)
	if err != nil {
		return &leaderelection.LeaderElector{}, err
	}

	leConfig := leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: cfg.LeaseDuration,
		RenewDeadline: cfg.RenewDeadline,
		RetryPeriod:   cfg.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: startFunc,
			OnStoppedLeading: stopFunc,
		},
	}

	return leaderelection.NewLeaderElector(leConfig)
}
