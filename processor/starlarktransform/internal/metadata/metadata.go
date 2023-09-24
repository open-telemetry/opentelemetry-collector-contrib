package metadata

import (
	"go.opentelemetry.io/collector/component"
)

const (
	Type             = "starlarktransform"
	LogsStability    = component.StabilityLevelAlpha
	MetricsStability = component.StabilityLevelAlpha
	TracesStability  = component.StabilityLevelAlpha
)
