package apachepulsarreceiver

import (
	"time"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	defaultCollectionInterval = 60 * time.Second
	defaultEndpoint           = "http://localhost:8080/admin/v2"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"` // JSON to Go struct "rules" (Go tag, used for marshaling/unmarshaling)
}
