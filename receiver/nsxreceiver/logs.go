// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nsxreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

var _ component.Receiver = (*nsxLogsReceiver)(nil)

type nsxLogsReceiver struct {
	cfg            *Config
	params         component.ReceiverCreateSettings
	consumer       consumer.Logs
	syslogReceiver component.LogsReceiver
}

func newLogsReceiver(c *Config, params component.ReceiverCreateSettings, consumer consumer.Logs) *nsxLogsReceiver {
	return &nsxLogsReceiver{
		cfg:      c,
		params:   params,
		consumer: consumer,
	}
}

func (lr *nsxLogsReceiver) Start(ctx context.Context, h component.Host) error {
	f := h.GetFactory(component.KindReceiver, "syslog")
	rf, ok := f.(component.ReceiverFactory)
	if !ok {
		return fmt.Errorf("unable to wrap the syslog receiver that the %s component wraps", typeStr)
	}
	syslog, err := rf.CreateLogsReceiver(ctx, lr.params, lr.cfg.LoggingConfig.SysLogConfig, lr.consumer)
	if err != nil {
		return fmt.Errorf("unable to start the wrapped syslogreceiver: %w", err)
	}
	lr.syslogReceiver = syslog
	return lr.syslogReceiver.Start(ctx, h)
}

func (lr *nsxLogsReceiver) Shutdown(ctx context.Context) error {
	if lr.syslogReceiver != nil {
		return lr.syslogReceiver.Shutdown(ctx)
	}
	return nil
}
