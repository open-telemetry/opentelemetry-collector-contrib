// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
)

// NewFactory creates a factory for AWS-Xray exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		AWSSessionSettings: awsutil.CreateDefaultSessionConfig(),
	}
}

func createTracesExporter(
	_ context.Context,
	params exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	eCfg := cfg.(*Config)
	return newTracesExporter(eCfg, params, &awsutil.Conn{}, telemetry.GlobalRegistry())
}
