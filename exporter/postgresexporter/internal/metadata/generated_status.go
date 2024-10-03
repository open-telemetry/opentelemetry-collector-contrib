package metadata

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type      = component.MustNewType("postgres")
	ScopeName = "postgresexporter"
)

const (
	TracesStability  = component.StabilityLevelAlpha
	LogsStability    = component.StabilityLevelAlpha
	MetricsStability = component.StabilityLevelAlpha
)
