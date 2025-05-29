// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
)

var skipTimestampValidationFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.awsxray.skiptimestampvalidation",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("Remove XRay's timestamp validation on first 32 bits of trace ID"),
	featuregate.WithRegisterFromVersion("v0.84.0"))

// NewFactory creates a factory for AWS-Xray exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		AWSSessionSettings:      awsutil.CreateDefaultSessionConfig(),
		skipTimestampValidation: skipTimestampValidationFeatureGate.IsEnabled(),
	}
}

func createTracesExporter(ctx context.Context, params exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	eCfg := cfg.(*Config)
	return newTracesExporter(ctx, eCfg, params, telemetry.GlobalRegistry())
}
