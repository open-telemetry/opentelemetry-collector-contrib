package metadata

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type      = component.MustNewType("postgresexporter")
	ScopeName = "postgresexporter"
)

const (
	TracesStability  = component.StabilityLevelAlpha
	LogsStability    = component.StabilityLevelAlpha
	MetricsStability = component.StabilityLevelAlpha
)
