// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

// This file implements Factory for Array scraper.

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal/metadata"
)

// NewFactory creates a factory for Pure Storage FlashArray receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ArrayName:    "foobar.example.com",
		Namespace:    "purefa",
		ClientConfig: confighttp.NewDefaultClientConfig(),
		Settings: &Settings{
			ReloadIntervals: &ReloadIntervals{
				Array:       60 * time.Second,
				Hosts:       60 * time.Second,
				Directories: 60 * time.Second,
				Pods:        60 * time.Second,
				Volumes:     60 * time.Second,
			},
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	rCfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := rCfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("a purefa receiver config was expected by the receiver factory, but got %T", rCfg)
	}
	return newReceiver(cfg, set, next), nil
}
