// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type rawLogsUnmarshaler struct {
	logger *zap.Logger
}

func newRawLogsUnmarshaler(logger *zap.Logger) eventLogsUnmarshaler {

	return rawLogsUnmarshaler{
		logger: logger,
	}
}

func (r rawLogsUnmarshaler) UnmarshalLogs(event *eventhub.Event) (plog.Logs, error) {

	l := plog.NewLogs()
	lr := l.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	slice := lr.Body().SetEmptyBytes()
	slice.Append(event.Data...)
	if event.SystemProperties.EnqueuedTime != nil {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(*event.SystemProperties.EnqueuedTime))
	}

	if err := lr.Attributes().FromRaw(event.Properties); err != nil {
		return l, err
	}

	return l, nil
}
