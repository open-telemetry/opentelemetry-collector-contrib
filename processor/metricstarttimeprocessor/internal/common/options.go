package common

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
)

type AdjustmentOptions struct {
	Filter         filterset.FilterSet
	SkipIfCTExists bool
	GCInterval     time.Duration
}
