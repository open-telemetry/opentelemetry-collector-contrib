// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package custombalancer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

type testConfig struct{}

func (testConfig) Validate() error { return nil }

type noopLogsReceiver struct{}

func (noopLogsReceiver) Start(context.Context, component.Host) error { return nil }
func (noopLogsReceiver) Shutdown(context.Context) error              { return nil }

func TestContextWithGroupBalancerResolver_NilContext(t *testing.T) {
	resolver := func(configkafka.GroupRebalanceStrategy) ([]kgo.GroupBalancer, bool, error) {
		return nil, false, nil
	}

	ctx := ContextWithGroupBalancerResolver(nil, resolver) //nolint:staticcheck
	require.NotNil(t, ctx)
	require.NotNil(t, GroupBalancerResolverFromContext(ctx))
}

func TestContextWithGroupBalancerResolver_NilResolver(t *testing.T) {
	ctx := ContextWithGroupBalancerResolver(t.Context(), nil)
	require.NotNil(t, ctx)
	require.Nil(t, GroupBalancerResolverFromContext(ctx))
}

func TestWrapFactory_InjectsResolverIntoContext(t *testing.T) {
	resolverCalled := false
	resolver := func(configkafka.GroupRebalanceStrategy) ([]kgo.GroupBalancer, bool, error) {
		resolverCalled = true
		return nil, false, nil
	}

	base := xreceiver.NewFactory(
		component.MustNewType("custombalancer_test"),
		func() component.Config { return testConfig{} },
		xreceiver.WithLogs(func(ctx context.Context, _ receiver.Settings, _ component.Config, _ consumer.Logs) (receiver.Logs, error) {
			fromCtx := GroupBalancerResolverFromContext(ctx)
			require.NotNil(t, fromCtx)

			_, _, err := fromCtx("private-strategy")
			require.NoError(t, err)
			return noopLogsReceiver{}, nil
		}, component.StabilityLevelDevelopment),
	)

	wrapped := WrapFactory(base, resolver)
	nextLogs, err := consumer.NewLogs(func(context.Context, plog.Logs) error { return nil })
	require.NoError(t, err)

	set := receiver.Settings{ID: component.NewID(component.MustNewType("custombalancer_test"))}
	_, err = wrapped.CreateLogs(t.Context(), set, wrapped.CreateDefaultConfig(), nextLogs)
	require.NoError(t, err)
	require.True(t, resolverCalled)
}
