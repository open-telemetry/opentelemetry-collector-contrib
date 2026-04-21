// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package featuregates // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/featuregates"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/internal/metadata"

// Re-exported from generated code for backward compatibility.
var (
	ReceiveResourceSpansV2FeatureGate          = metadata.DatadogEnableReceiveResourceSpansV2FeatureGate
	OperationAndResourceNameV2FeatureGate      = metadata.DatadogEnableOperationAndResourceNameV2FeatureGate
	MetricRemappingDisabledFeatureGate         = metadata.ExporterDatadogexporterMetricremappingdisabledFeatureGate
	DisableMetricRemappingFeatureGate          = metadata.ExporterDatadogexporterDisableAllMetricRemappingFeatureGate
	AttributeSliceMultiTagExportingFeatureGate = metadata.ExporterDatadogexporterEnableAttributeSliceMultiTagExportingFeatureGate
	InferIntervalDeltaFeatureGate              = metadata.ExporterDatadogexporterInferIntervalForDeltaMetricsFeatureGate
)
