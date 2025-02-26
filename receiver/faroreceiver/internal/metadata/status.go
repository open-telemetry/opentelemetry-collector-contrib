package metadata

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type      = component.MustNewType("faro")
	ScopeName = "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"
)

const (
	TracesStability = component.StabilityLevelDevelopment
	LogsStability   = component.StabilityLevelDevelopment
)
