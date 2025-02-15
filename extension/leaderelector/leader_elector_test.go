// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLeaderElector(t *testing.T) {
	fakeClient := fake.NewClientset()
	onStartedLeading := func(ctx context.Context) {}
	onStoppedLeading := func() {}
	leConfig := Config{
		LeaseName:      "foo",
		LeaseNamespace: "bar",
		LeaseDuration:  20 * time.Second,
		RenewDuration:  10 * time.Second,
		RetryPeriod:    2 * time.Second,
	}

	leaderElector, err := NewLeaderElector(&leConfig, fakeClient, onStartedLeading, onStoppedLeading, "host1")
	require.NoError(t, err)
	require.NotNil(t, leaderElector)
}
