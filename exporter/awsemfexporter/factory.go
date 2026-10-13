// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// NewFactory creates a factory for AWS EMF exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

// CreateDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() component.Config {
	var defaultDimensionRollupOption string
	if metadata.AwsemfNodimrollupdefaultFeatureGate.IsEnabled() {
		defaultDimensionRollupOption = "NoDimensionRollup"
	} else {
		defaultDimensionRollupOption = "ZeroAndSingleDimensionRollup"
	}
	return &Config{
		AWSSessionSettings:              awsutil.CreateDefaultSessionConfig(),
		LogGroupName:                    "",
		LogStreamName:                   "",
		Namespace:                       "",
		DimensionRollupOption:           defaultDimensionRollupOption,
		Version:                         "1",
		RetainInitialValueOfDeltaMetric: false,
		OutputDestination:               "cloudwatch",
		logger:                          zap.NewNop(),
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(ctx context.Context, params exporter.Settings, config component.Config) (exporter.Metrics, error) {
	expCfg := config.(*Config)

	emfExp, err := newEmfExporter(ctx, expCfg, params)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetrics(
		ctx,
		params,
		config,
		emfExp.pushMetricsData,
		exporterhelper.WithShutdown(emfExp.shutdown),
	)
	if err != nil {
		return nil, err
	}

	rttSettings := expCfg.ResourceToTelemetrySettings
	//nolint:staticcheck // check deprecated fields
	if (rttSettings.Enabled || rttSettings.ExcludeServiceAttributes) && !metadata.ExporterAwsemfDisableLegacyResourceToTelemetryConversionFeatureGate.IsEnabled() {
		params.Logger.Warn("enabled and exclude_service_attributes in resource_to_telemetry_conversion are deprecated; use included and excluded patterns instead")
	}
	if metadata.ExporterAwsemfDisableLegacyResourceToTelemetryConversionFeatureGate.IsEnabled() {
		rttSettings.Enabled = false                  //nolint:staticcheck // ignore deprecated field
		rttSettings.ExcludeServiceAttributes = false //nolint:staticcheck // ignore deprecated field
	}

	return resourcetotelemetry.WrapMetricsExporter(rttSettings, exporter), nil
}
