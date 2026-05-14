// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/xextension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return xextension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
		xextension.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createExtension(_ context.Context, _ extension.Settings, config component.Config) (extension.Extension, error) {
	return newExtension(config.(*Config)), nil
}

func createDefaultConfig() component.Config {
	return &Config{
		HandleJSONPayloadAs:  HandleAsJSON,
		HandleProtoPayloadAs: HandleAsJSON,
	}
}
