// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"

import "go.opentelemetry.io/collector/featuregate"

// ReceiveResourceSpansV2FeatureGate is a feature gate that enables a refactored implementation of span processing in Datadog exporter and connector
var ReceiveResourceSpansV2FeatureGate = featuregate.GlobalRegistry().MustRegister(
	"datadog.EnableReceiveResourceSpansV2",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled, use a refactored implementation of the span receiver which improves performance by 10% and deprecates some not-to-spec functionality."),
	featuregate.WithRegisterFromVersion("v0.118.0"),
)

// OperationAndResourceNameV2FeatureGate is a feature gate that enables enhanced span operation name and resource names in Datadog exporter and connector
var OperationAndResourceNameV2FeatureGate = featuregate.GlobalRegistry().MustRegister(
	"datadog.EnableOperationAndResourceNameV2",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled, datadogexporter and datadogconnector use improved logic to compute operation name and resource name."),
	featuregate.WithRegisterFromVersion("v0.118.0"),
)

// MetricRemappingDisabledFeatureGate is a feature gate that controls the client-side mapping from OpenTelemetry semantic conventions to Datadog semantic conventions
var MetricRemappingDisabledFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.datadogexporter.metricremappingdisabled",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled the Datadog Exporter stops mapping OpenTelemetry semantic conventions to Datadog semantic conventions. This feature gate is only for internal use."),
	featuregate.WithRegisterReferenceURL("https://docs.datadoghq.com/opentelemetry/schema_semantics/metrics_mapping/"),
)
