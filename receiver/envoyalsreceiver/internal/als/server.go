// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package als // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver/internal/als"

import (
	"context"
	"errors"
	"io"

	alsv3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	apiVersionAttr = "api_version"
	apiVersionVal  = "v3"
	logTypeAttr    = "log_type"
	httpTypeVal    = "http"
	tcpTypeVal     = "tcp"
)

type Server struct {
	nextConsumer consumer.Logs
	obsrep       *receiverhelper.ObsReport
}

func New(nextConsumer consumer.Logs, obsrep *receiverhelper.ObsReport) *Server {
	return &Server{
		nextConsumer: nextConsumer,
		obsrep:       obsrep,
	}
}

func (s *Server) StreamAccessLogs(logStream alsv3.AccessLogService_StreamAccessLogsServer) error {
	for {
		data, err := logStream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		ctx := s.obsrep.StartLogsOp(context.Background())
		logs := toLogs(data)
		logRecordCount := logs.LogRecordCount()
		err = s.nextConsumer.ConsumeLogs(ctx, logs)
		s.obsrep.EndLogsOp(ctx, "protobuf", logRecordCount, err)
		if err != nil {
			return err
		}
	}

	return nil
}

func toLogs(data *alsv3.StreamAccessLogsMessage) plog.Logs {
	logs := plog.NewLogs()

	rls := logs.ResourceLogs().AppendEmpty()
	logSlice := rls.ScopeLogs().AppendEmpty().LogRecords()

	httpLogs := data.GetHttpLogs()
	if httpLogs != nil {
		for _, httpLog := range httpLogs.LogEntry {
			lr := logSlice.AppendEmpty()
			lr.SetTimestamp(pcommon.NewTimestampFromTime(httpLog.CommonProperties.StartTime.AsTime()))
			lr.Attributes().PutStr(apiVersionAttr, apiVersionVal)
			lr.Attributes().PutStr(logTypeAttr, httpTypeVal)
			lr.Body().SetStr(httpLog.String())
		}
	}

	tcpLogs := data.GetTcpLogs()
	if tcpLogs != nil {
		for _, tcpLog := range tcpLogs.LogEntry {
			lr := logSlice.AppendEmpty()
			lr.SetTimestamp(pcommon.NewTimestampFromTime(tcpLog.CommonProperties.StartTime.AsTime()))
			lr.Attributes().PutStr(apiVersionAttr, apiVersionVal)
			lr.Attributes().PutStr(logTypeAttr, tcpTypeVal)
			lr.Body().SetStr(tcpLog.String())
		}
	}
	return logs
}
