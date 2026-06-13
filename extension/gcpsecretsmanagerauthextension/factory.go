// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcpsecretsmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/gcpsecretsmanagerauthextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/gcpsecretsmanagerauthextension/internal/metadata"
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
	return &Config{}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	c := cfg.(*Config)
	if c.Htpasswd != nil {
		return newServerAuthExtension(c.Htpasswd, set.Logger), nil
	}
	return newClientAuthExtension(c.ClientAuth, set.Logger), nil
}
