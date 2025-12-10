package metadata

import (
	"go.opentelemetry.io/collector/component"
)

const (
	typ              = "awsecsattributes"
	TracesStability  = component.StabilityLevelBeta
	MetricsStability = component.StabilityLevelBeta
	LogsStability    = component.StabilityLevelStable
)

var (
	Type, _ = component.NewType(typ)
)
