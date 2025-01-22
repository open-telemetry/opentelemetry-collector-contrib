// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"

import "go.opentelemetry.io/collector/featuregate"

var ReceiveResourceSpansV2FeatureGate = featuregate.GlobalRegistry().MustRegister(
	"datadog.EnableReceiveResourceSpansV2",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, use a refactored implementation of the span receiver which improves performance by 10% and deprecates some not-to-spec functionality."),
	featuregate.WithRegisterFromVersion("v0.118.0"),
	featuregate.WithRegisterToVersion("v0.124.0"),
)

var OperationAndResourceNameV2FeatureGate = featuregate.GlobalRegistry().MustRegister(
	"datadog.EnableOperationAndResourceNameV2",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, datadogexporter and datadogconnector use improved logic to compute operation name and resource name."),
	featuregate.WithRegisterFromVersion("v0.118.0"),
)
