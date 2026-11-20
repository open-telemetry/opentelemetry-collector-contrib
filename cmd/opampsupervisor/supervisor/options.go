// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon/fileaddon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon/jmxaddon"
)

// Option customizes a Supervisor built by a distribution.
type Option func(*options)

type options struct {
	addonFactories []addon.Factory
}

// WithAddonFactories adds addon types to the ones the Supervisor ships with.
// A distribution uses it to support addon types specific to it:
//
//	sup, err := supervisor.NewSupervisor(ctx, logger, cfg,
//		supervisor.WithAddonFactories(customrules.NewFactory()))
//
// Registering a type that is already supported is an error.
func WithAddonFactories(factories ...addon.Factory) Option {
	return func(o *options) {
		o.addonFactories = append(o.addonFactories, factories...)
	}
}

// builtinAddonFactories are the addon types every Supervisor supports.
func builtinAddonFactories() []addon.Factory {
	return []addon.Factory{
		fileaddon.NewFactory(),
		jmxaddon.NewFactory(),
	}
}
