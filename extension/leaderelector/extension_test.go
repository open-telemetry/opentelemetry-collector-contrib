// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"
	"fmt"
	"testing"
	"time"

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

func TestMultipleExtension(t *testing.T) {
	config := &Config{
		LeaseName:      "foo",
		LeaseNamespace: "default",
		LeaseDuration:  15 * time.Second,
		RenewDuration:  10 * time.Second,
		RetryPeriod:    2 * time.Second,
	}

	iamInvokedOnLeading := false

	ctx := context.TODO()
	fakeClient := fake.NewClientset()
	config.makeClient = func(apiConfig k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return fakeClient, nil
	}

	observedZapCore, _ := observer.New(zap.WarnLevel)

	leaderElection := leaderElectionExtension{
		config: config,
		client: fakeClient,
		logger: zap.New(observedZapCore),
	}

	leaderElection.SetCallBackFuncs(
		func(ctx context.Context) {
			iamInvokedOnLeading = true
			fmt.Printf("LeaderElection started leading")
		},
		func() {
			fmt.Printf("LeaderElection stopped leading")
		},
	)

	require.NoError(t, leaderElection.Start(ctx, componenttest.NewNopHost()))

	expectedLeaseDurationSeconds := ptr.To(int32(15))

	require.Eventually(t, func() bool {
		lease, err := fakeClient.CoordinationV1().Leases("default").Get(ctx, "foo", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, lease)
		require.Equal(t, expectedLeaseDurationSeconds, lease.Spec.LeaseDurationSeconds)
		return true
	}, 10*time.Second, 100*time.Millisecond)

	require.True(t, iamInvokedOnLeading)
	require.NoError(t, leaderElection.Shutdown(ctx))
}
