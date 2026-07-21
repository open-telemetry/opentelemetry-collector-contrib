// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth/awsiamdbauthextension"

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth/awsiamdbauthextension/internal/metadata"
)

// NewFactory returns the aws_iam credential provider as a Collector extension
// factory. The extension is declared once (and listed in service.extensions) and
// is referenced by any number of receivers, each naming it by component ID inside
// their db_auth block. It holds the provider-wide config (the required region);
// the per-connection mint inputs (endpoint, db user) arrive with each
// GetCredential call, so one declared extension serves many receivers
// concurrently. To vary the region, declare multiple named instances.
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

func createExtension(ctx context.Context, _ extension.Settings, cfg component.Config) (extension.Extension, error) {
	c, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("aws_iam: unexpected config type %T", cfg)
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(c.Region))
	if err != nil {
		return nil, fmt.Errorf("aws_iam: load AWS config: %w", err)
	}
	return &iamExtension{
		cfg:       c,
		awsConfig: awsCfg,
	}, nil
}
