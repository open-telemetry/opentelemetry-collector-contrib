// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelector

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestExtension(t *testing.T) {
	config := &Config{
		LeaseName:      "foo",
		LeaseNamespace: "default",
		LeaseDuration:  15 * time.Second,
		RenewDuration:  10 * time.Second,
		RetryPeriod:    2 * time.Second,
	}

	ctx := t.Context()
	fakeClient := fake.NewClientset()
	config.makeClient = func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return fakeClient, nil
	}

	observedZapCore, _ := observer.New(zap.WarnLevel)

	leaderElection := leaderElectionExtension{
		config:        config,
		client:        fakeClient,
		logger:        zap.New(observedZapCore),
		leaseHolderID: "foo",
	}

	var onStartLeadingInvoked atomic.Bool
	leaderElection.SetCallBackFuncs(
		func(_ context.Context) {
			onStartLeadingInvoked.Store(true)
			fmt.Printf("LeaderElection started leading")
		},
		func() {
			fmt.Printf("LeaderElection stopped leading")
		},
	)

	require.NoError(t, leaderElection.Start(ctx, componenttest.NewNopHost()))

	expectedLeaseDurationSeconds := ptr.To(int32(15))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		lease, err := fakeClient.CoordinationV1().Leases("default").Get(ctx, "foo", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, lease)
		require.Equal(t, expectedLeaseDurationSeconds, lease.Spec.LeaseDurationSeconds)
		require.True(t, onStartLeadingInvoked.Load())
	}, 10*time.Second, 100*time.Millisecond)

	require.NoError(t, leaderElection.Shutdown(ctx))
}

func TestExtension_WithDelay(t *testing.T) {
	config := &Config{
		LeaseName:      "foo",
		LeaseNamespace: "default",
		LeaseDuration:  15 * time.Second,
		RenewDuration:  10 * time.Second,
		RetryPeriod:    2 * time.Second,
	}

	ctx := t.Context()
	fakeClient := fake.NewClientset()
	config.makeClient = func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return fakeClient, nil
	}

	observedZapCore, _ := observer.New(zap.WarnLevel)

	leaderElection := leaderElectionExtension{
		config:        config,
		client:        fakeClient,
		logger:        zap.New(observedZapCore),
		leaseHolderID: "foo",
	}

	var onStartLeadingInvoked atomic.Bool

	require.NoError(t, leaderElection.Start(ctx, componenttest.NewNopHost()))

	// Simulate a delay of setting up callbacks after the leader has been elected.
	expectedLeaseDurationSeconds := ptr.To(int32(15))
	// TODO: Remove time.Sleep below, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42460
	time.Sleep(100 * time.Millisecond)
	require.Eventually(t, func() bool {
		lease, err := fakeClient.CoordinationV1().Leases("default").Get(ctx, "foo", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, lease)
		require.NotNil(t, lease.Spec.AcquireTime)
		require.NotNil(t, lease.Spec.HolderIdentity)
		require.Equal(t, expectedLeaseDurationSeconds, lease.Spec.LeaseDurationSeconds)
		return true
	}, 10*time.Second, 100*time.Millisecond)

	leaderElection.SetCallBackFuncs(
		func(_ context.Context) {
			onStartLeadingInvoked.Store(true)
			fmt.Printf("%v: LeaderElection started leading\n", time.Now().String())
		},
		func() {
			fmt.Printf("%v: LeaderElection stopped leading\n", time.Now().String())
		},
	)

	require.True(t, onStartLeadingInvoked.Load())
	require.NoError(t, leaderElection.Shutdown(ctx))
}
