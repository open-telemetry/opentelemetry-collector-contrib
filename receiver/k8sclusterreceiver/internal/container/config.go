// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/container/internal/metadata"
)

// Config relating to CPU Metric Scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

func CreateDefaultConfig() Config {
	return Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}
