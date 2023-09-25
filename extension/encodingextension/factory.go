// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension/internal/text"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/codec"
)

// NewFactory creates a factory for the encoding extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	textLogsUnmarshaler := text.NewLogCodec("utf8")
	return &Config{
		LogCodecs: map[string]codec.Log{
			"text": textLogsUnmarshaler,
		},
	}
}

func createExtension(_ context.Context, _ extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	return &Extension{
		config: cfg.(*Config),
	}, nil
}
