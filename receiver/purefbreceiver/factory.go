// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver"

// This file implements Factory for Array scraper.

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver/internal/metadata"
)

// NewFactory creates a factory for Pure Storage FlashBlade receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{},
		Settings: &Settings{
			ReloadIntervals: &ReloadIntervals{
				Array:   15 * time.Second,
				Clients: 5 * time.Minute,
				Usage:   5 * time.Minute,
			},
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	rCfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := rCfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("a purefb receiver config was expected by the receiver factory, but got %T", rCfg)
	}
	return newReceiver(cfg, set, next), nil
}
