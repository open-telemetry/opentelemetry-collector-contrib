// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		GoMaxProcs: GoMaxProcsConfig{
			Enable: true,
		},
		GoMemLimit: GoMemLimitConfig{
			Enable: true,
			// By default, it sets `GOMEMLIMIT` to 90% of cgroup's memory limit.
			Ratio: 0.9,
		},
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newCgroupRuntime(cfg.(*Config), set.TelemetrySettings), nil
}
