package vmwarevcenterreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

var _ component.Receiver = (*vcenterLogsReceiver)(nil)

type vcenterLogsReceiver struct {
	cfg            *Config
	params         component.ReceiverCreateSettings
	consumer       consumer.Logs
	tcpLogReceiver component.LogsReceiver
}

func newLogsReceiver(c *Config, params component.ReceiverCreateSettings, consumer consumer.Logs) *vcenterLogsReceiver {
	return &vcenterLogsReceiver{
		cfg:      c,
		params:   params,
		consumer: consumer,
	}
}

func (lr *vcenterLogsReceiver) Start(ctx context.Context, h component.Host) error {
	f := h.GetFactory(component.KindReceiver, "tcplog")
	rf, ok := f.(component.ReceiverFactory)
	if !ok {
		return fmt.Errorf("unable to wrap the tcplog receiver that the %s component wraps", typeStr)
	}
	tcp, err := rf.CreateLogsReceiver(ctx, lr.params, lr.cfg.LoggingConfig.TCPLogConfig, lr.consumer)
	if err != nil {
		return fmt.Errorf("unable to start the wrapped tcplog receiver: %w", err)
	}
	lr.tcpLogReceiver = tcp
	return lr.tcpLogReceiver.Start(ctx, h)
}

func (lr *vcenterLogsReceiver) Shutdown(ctx context.Context) error {
	if lr.tcpLogReceiver != nil {
		return lr.tcpLogReceiver.Shutdown(ctx)
	}
	return nil
}
