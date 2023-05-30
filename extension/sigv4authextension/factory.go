// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension/internal/metadata"
)

// NewFactory creates a factory for the Sigv4 Authenticator extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(metadata.Type, createDefaultConfig, createExtension, metadata.ExtensionStability)
}

// createDefaultConfig() creates a Config struct with default values.
// We only set the ID here.
func createDefaultConfig() component.Config {
	return &Config{}
}

// createExtension() calls newSigv4Extension() in extension.go to create the extension.
func createExtension(_ context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	awsSDKInfo := fmt.Sprintf("%s/%s", aws.SDKName, aws.SDKVersion)
	return newSigv4Extension(cfg.(*Config), awsSDKInfo, set.Logger), nil
}
