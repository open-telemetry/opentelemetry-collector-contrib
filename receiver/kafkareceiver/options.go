// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// GroupBalancerResolver resolves custom group rebalance strategy values into
// franz-go balancers.
//
// The resolver is consulted only when a strategy is not one of the built-in
// values ("range", "roundrobin", "sticky", "cooperative-sticky").
//
// ok=false indicates the resolver does not provide balancers for the strategy.
type GroupBalancerResolver func(strategy configkafka.GroupRebalanceStrategy) ([]kgo.GroupBalancer, bool, error)

// FactoryOption configures the Kafka receiver factory.
type FactoryOption func(*factoryOptions)

type factoryOptions struct {
	groupBalancerResolver GroupBalancerResolver
}

// WithGroupBalancerResolver enables custom group rebalance strategy resolution
// at factory construction time. This is intended for custom collector builds.
func WithGroupBalancerResolver(resolver GroupBalancerResolver) FactoryOption {
	return func(opts *factoryOptions) {
		opts.groupBalancerResolver = resolver
	}
}
