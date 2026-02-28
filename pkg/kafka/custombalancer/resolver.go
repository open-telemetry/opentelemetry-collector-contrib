// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package custombalancer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/custombalancer"

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// GroupBalancerResolver resolves a custom group rebalance strategy value into
// franz-go balancers. When group_rebalance_strategy is left unset, strategy is
// an empty string. ok=false indicates the strategy is not handled.
type GroupBalancerResolver func(configkafka.GroupRebalanceStrategy) ([]kgo.GroupBalancer, bool, error)

type resolverContextKey struct{}

// ContextWithGroupBalancerResolver stores a custom balancer resolver in the
// provided context.
func ContextWithGroupBalancerResolver(ctx context.Context,
	resolver GroupBalancerResolver,
) context.Context {
	return context.WithValue(ctx, resolverContextKey{}, resolver)
}

// GroupBalancerResolverFromContext returns the custom balancer resolver stored
// in the context, if any.
func GroupBalancerResolverFromContext(ctx context.Context) GroupBalancerResolver {
	if ctx == nil {
		return nil
	}
	resolver, _ := ctx.Value(resolverContextKey{}).(GroupBalancerResolver)
	return resolver
}

// WrapFactory wraps a receiver factory and injects the resolver into the
// context for all Create* methods when the provided factory supports xreceiver.
func WrapFactory(base receiver.Factory, r GroupBalancerResolver) receiver.Factory {
	if base == nil || r == nil {
		return base
	}
	xf, ok := base.(xreceiver.Factory)
	if !ok {
		return base
	}
	return wrappedFactory{
		Factory:  xf,
		resolver: r,
	}
}

type wrappedFactory struct {
	xreceiver.Factory
	resolver GroupBalancerResolver
}

func (f wrappedFactory) CreateTraces(ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Traces,
) (receiver.Traces, error) {
	return f.Factory.CreateTraces(
		ContextWithGroupBalancerResolver(ctx, f.resolver), set, cfg, next,
	)
}

func (f wrappedFactory) CreateMetrics(ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	return f.Factory.CreateMetrics(
		ContextWithGroupBalancerResolver(ctx, f.resolver), set, cfg, next,
	)
}

func (f wrappedFactory) CreateLogs(ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	return f.Factory.CreateLogs(
		ContextWithGroupBalancerResolver(ctx, f.resolver), set, cfg, next,
	)
}

func (f wrappedFactory) CreateProfiles(ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	next xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	return f.Factory.CreateProfiles(
		ContextWithGroupBalancerResolver(ctx, f.resolver), set, cfg, next,
	)
}
