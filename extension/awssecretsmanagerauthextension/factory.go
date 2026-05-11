// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension/internal/metadata"
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
		RefreshInterval: 30 * time.Second,
		ClientAuth:      &ClientAuthSettings{},
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	c := cfg.(*Config)
	if c.Htpasswd != nil {
		return newServerAuth(c, set.Logger), nil
	}
	return newClientAuth(c, set.Logger), nil
}
