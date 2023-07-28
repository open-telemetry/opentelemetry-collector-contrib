// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sconfig

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"os"
	"time"
)

// LeaderElectionConfig is used to enable leader election
type LeaderElectionConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	LeaderElectionID string `mapstructure:"leader_election_id"`
}

const (
	inClusterNamespacePath  = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultLeaderElectionID = "opentelemetry-collector"
	defaultNamespace        = "default"
	defaultLeaseDuration    = 15 * time.Second
	defaultRenewDeadline    = 10 * time.Second
	defaultRetryPeriod      = 2 * time.Second
)

// NewResourceLock creates a new config map resource lock for use in a leader
// election loop
func newResourceLock(client kubernetes.Interface, leaderElectionID string) (resourcelock.Interface, error) {
	if leaderElectionID == "" {
		leaderElectionID = defaultLeaderElectionID
	}

	leaderElectionNamespace, err := getInClusterNamespace()
	if err != nil {
		leaderElectionNamespace = defaultNamespace
	}

	// Leader id, needs to be unique
	id, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	id = id + "_" + uuid.New().String()

	return resourcelock.New(resourcelock.ConfigMapsLeasesResourceLock,
		leaderElectionNamespace,
		leaderElectionID,
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
		return "", fmt.Errorf("not running in-cluster, please specify leaderElectionIDspace")
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
func NewLeaderElector(leaderElectionID string, client kubernetes.Interface, startFunc func(context.Context), stopFunc func()) (*leaderelection.LeaderElector, error) {
	resourceLock, err := newResourceLock(client, leaderElectionID)
	if err != nil {
		return &leaderelection.LeaderElector{}, err
	}

	// TODO: make those configurable.
	l, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: defaultLeaseDuration,
		RenewDeadline: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: startFunc,
			OnStoppedLeading: stopFunc,
		},
	})
	return l, err
}
