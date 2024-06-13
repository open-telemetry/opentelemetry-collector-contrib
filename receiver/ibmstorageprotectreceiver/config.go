package ibmstorageprotectreceiver

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"go.opentelemetry.io/collector/component"
)

const (
	transport       = "file"
	compressionZSTD = "zstd"
	formatTypeJSON  = "json"
)

// Specifying component dealing with metrics data
const (
	MetricsStability = component.StabilityLevelAlpha
)

type Config struct {
	fileconsumer.Config `mapstructure:",squash"`
	StorageID           *component.ID `mapstructure:",storage"`
}
