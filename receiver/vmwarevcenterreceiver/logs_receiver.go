package vmwarevcenterreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
)

type logsReceiver struct {
	cfg *Config
}

func newLogsReceiver(c *Config) *logsReceiver {
	return &logsReceiver{
		cfg: c,
	}
}

func (lr *logsReceiver) Start(_ context.Context, h component.Host) error {
	h.GetFactory(component.KindReceiver, "tcp")
	return nil
}

func (lr *logsReceiver) Shutdown(_ context.Context) error {
	return nil
}
