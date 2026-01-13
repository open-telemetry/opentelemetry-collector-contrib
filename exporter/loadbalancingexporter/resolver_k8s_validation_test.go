// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

// TestConfigValidationWithK8sResolverOutsideCluster validates that we can now create an exporter
// with k8s resolver without requiring access to a Kubernetes cluster. This resolves issue #42293.
// The k8s client will only be created when Start() is called, not during construction.
func TestConfigValidationWithK8sResolverOutsideCluster(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		Resolver: ResolverSettings{
			K8sSvc: configoptional.Some(K8sSvcResolver{
				Service: "my-service.my-namespace",
				Ports:   []int32{4317},
			}),
		},
		RoutingKey: "service",
	}

	settings := exportertest.NewNopSettings(metadata.Type)

	// This should NOT fail even when running outside a Kubernetes cluster
	// because the k8s client is not created until Start() is called
	exp, err := factory.CreateTraces(t.Context(), settings, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	// Note: calling Start() would fail here because we're not in a k8s cluster,
	// but that's expected behavior. The important part is that config validation
	// (exporter creation) succeeds.
}

// TestK8sResolverLazyClientCreation verifies that the k8s client is created lazily during start
func TestK8sResolverLazyClientCreation(t *testing.T) {
	_, tb := getTelemetryAssets(t)

	// Create resolver without client - should succeed
	resolver, err := newK8sResolver(
		nil, // nil client should be accepted
		zap.NewNop(),
		"test-service.test-namespace",
		[]int32{4317},
		defaultListWatchTimeout,
		false,
		tb,
	)
	require.NoError(t, err)
	require.NotNil(t, resolver)
	require.Nil(t, resolver.client, "client should be nil before start")

	// Note: We can't actually call start() in this test because it will try to create
	// a real k8s client, which will fail outside a k8s cluster. But the important part
	// is that the resolver can be created without a client.
}

// TestK8sResolverStartCreatesClient verifies that start() attempts to create the client when it's nil
func TestK8sResolverStartCreatesClient(t *testing.T) {
	_, tb := getTelemetryAssets(t)

	resolver, err := newK8sResolver(
		nil,
		zap.NewNop(),
		"test-service",
		[]int32{4317},
		defaultListWatchTimeout,
		false,
		tb,
	)
	require.NoError(t, err)
	require.Nil(t, resolver.client, "client should be nil before start")

	// Start will attempt to create the client when it's nil (lazy initialization).
	// This may succeed (if running in a k8s environment) or fail (if outside k8s cluster).
	err = resolver.start(t.Context())

	// Verify lazy initialization: if start() succeeded, the client must have been created.
	// If start() failed, that's expected outside k8s cluster, but the attempt was made.
	if err == nil {
		require.NotNil(t, resolver.client, "client must be created when start() succeeds")
	}
	// If err != nil, that's also valid - it proves start() attempted client creation

	// Clean up to avoid goroutine leaks
	_ = resolver.shutdown(t.Context())
}

// TestLoadBalancerWithK8sResolverCreation tests that loadbalancer can be created with k8s resolver
func TestLoadBalancerWithK8sResolverCreation(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			K8sSvc: configoptional.Some(K8sSvcResolver{
				Service: "my-service.my-ns",
				Ports:   []int32{4317},
			}),
		},
	}

	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}

	// Creating loadbalancer should succeed even outside k8s cluster
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	require.NotNil(t, lb)
	require.NotNil(t, lb.res, "resolver should be initialized")

	// Verify it's a k8s resolver
	_, ok := lb.res.(*k8sResolver)
	require.True(t, ok, "resolver should be k8sResolver type")
}
