package metadata

import (
	"go.opentelemetry.io/collector/component"
)

const (
	Type             = "pytransform"
	LogsStability    = component.StabilityLevelAlpha
	MetricsStability = component.StabilityLevelAlpha
	TracesStability  = component.StabilityLevelAlpha
)
