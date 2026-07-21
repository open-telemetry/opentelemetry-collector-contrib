// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscredentialsproviderextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscredentialsproviderextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscredentialsproviderextension/internal/metadata"
)

// NewFactory creates a factory for the AWS Authenticator extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(metadata.Type, createDefaultConfig, createExtension, metadata.ExtensionStability)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(_ context.Context, _ extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newAWSCredentialsProviderExtension(cfg.(*Config)), nil
}
