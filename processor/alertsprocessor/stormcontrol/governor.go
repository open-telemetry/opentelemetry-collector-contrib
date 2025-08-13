package stormcontrol

import (
	"time"

	"github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/evaluation"
)

type Governor struct{ cfg interface{} }

func New(cfg interface{}) *Governor { return &Governor{cfg: cfg} }

func (g *Governor) Adapt(_ **time.Ticker, _ []evaluation.Result, _ time.Time) {
	// no-op adapter for now
}
