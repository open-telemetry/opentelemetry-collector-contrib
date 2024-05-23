// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	// "go.opentelemetry.io/collector/pdata/pcommon"
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

func (r rawLogsUnmarshaler) UnmarshalLogs(event *azeventhubs.ReceivedEventData) (plog.Logs, error) {
	r.logger.Debug("started unmarshaling logs", zap.Any("eventBody", event.Body))
	l := plog.NewLogs()
	lr := l.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	slice := lr.Body().SetEmptyBytes()
	slice.Append(event.Body...)
	if err := lr.Attributes().FromRaw(event.Properties); err != nil {
		r.logger.Error("failed extracting attributes from raw event properties", zap.Error(err))
		return l, err
	}
	r.logger.Debug("successfully unmarshaled logs", zap.Any("logRecords", lr))
	return l, nil
}
