# Kafka custom balancer

This package exposes compile-time extension hooks for Kafka group balancers used by the Kafka receiver.

Use `WrapFactory` to inject a `GroupBalancerResolver` into `kafkareceiver.NewFactory()` in custom collector distributions.
