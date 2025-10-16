package agentcomponents

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type = component.MustNewType("datadog")
)

const (
	TracesToMetricsStability = component.StabilityLevelBeta
	TracesToTracesStability  = component.StabilityLevelBeta
)
