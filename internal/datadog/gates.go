package datadog

import "go.opentelemetry.io/collector/featuregate"

var ReceiveResourceSpansV2FeatureGate = featuregate.GlobalRegistry().MustRegister(
	"datadog.EnableReceiveResourceSpansV2",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, use a refactored implementation of the span receiver which improves performance by 10% and deprecates some not-to-spec functionality."),
	featuregate.WithRegisterFromVersion("v0.118.0"),
	featuregate.WithRegisterToVersion("v0.124.0"),
)
