package vcenterreceiver

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
	syslogReceiver component.LogsReceiver
}

func newLogsReceiver(c *Config, params component.ReceiverCreateSettings, consumer consumer.Logs) *vcenterLogsReceiver {
	return &vcenterLogsReceiver{
		cfg:      c,
		params:   params,
		consumer: consumer,
	}
}

func (lr *vcenterLogsReceiver) Start(ctx context.Context, h component.Host) error {
	f := h.GetFactory(component.KindReceiver, "syslog")
	rf, ok := f.(component.ReceiverFactory)
	if !ok {
		return fmt.Errorf("unable to wrap the tcplog receiver that the %s component wraps", typeStr)
	}
	tcp, err := rf.CreateLogsReceiver(ctx, lr.params, lr.cfg.LoggingConfig.SysLogConfig, lr.consumer)
	if err != nil {
		return fmt.Errorf("unable to start the wrapped syslogreceiver: %w", err)
	}
	lr.syslogReceiver = tcp
	return lr.syslogReceiver.Start(ctx, h)
}

func (lr *vcenterLogsReceiver) Shutdown(ctx context.Context) error {
	if lr.syslogReceiver != nil {
		return lr.syslogReceiver.Shutdown(ctx)
	}
	return nil
}
