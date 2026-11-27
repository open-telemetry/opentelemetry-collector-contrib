// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor_test

import (
	"context"
	"log"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// rulesFactory stands in for an addon type a distribution implements. See
// customRulesFactory in addons_test.go for one that does work.
type rulesFactory struct{}

func (rulesFactory) Type() addon.Type { return "custom_rules" }

func (rulesFactory) Install(_ context.Context, set addon.InstallSettings) error {
	return addon.ExtractTarGz(set.Content, set.Dir)
}

func (rulesFactory) Create(context.Context, addon.Settings) (addon.Addon, error) {
	return nil, nil
}

// A distribution builds its own Supervisor binary and registers its addon types
// alongside the built-in ones. The Windows service path takes the same options
// through supervisor.NewSvcHandler.
func ExampleWithAddonFactories() {
	cfg, err := config.Load("supervisor.yaml")
	if err != nil {
		log.Fatal(err)
	}

	sup, err := supervisor.NewSupervisor(context.Background(), zap.NewNop(), cfg,
		supervisor.WithAddonFactories(rulesFactory{}))
	if err != nil {
		log.Fatal(err)
	}

	if err := sup.Start(context.Background()); err != nil {
		log.Fatal(err)
	}
}
