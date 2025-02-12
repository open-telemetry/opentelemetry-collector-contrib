// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudaomexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/huaweicloudaomexporter"

import (
	"context"
	"encoding/json"
	"github.com/huaweicloud/huaweicloud-lts-sdk-go/producer"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"time"
)

// newLogsExporter return a new LogService logs exporter.
func newLogsExporter(set exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	l := &logServiceLogsSender{
		logger: set.Logger,
	}

	var err error
	if l.client, err = newLogServiceClient(cfg.(*Config), set.Logger); err != nil {
		return nil, err
	}

	return exporterhelper.NewLogs(
		context.TODO(),
		set,
		cfg,
		l.pushLogsData)
}

type logServiceLogsSender struct {
	logger *zap.Logger
	client logServiceClient
}

func (s *logServiceLogsSender) pushLogsData(_ context.Context, md plog.Logs) error {
	var err error
	ltsLogs := logDataToLogService(md)
	logs := change2CommonLogs(ltsLogs)
	if len(logs) > 0 {
		err = s.client.sendLogs(logs)
	}
	return err
}

func change2CommonLogs(list []*ExtendLog) []*producer.Log {
	logs := make([]*producer.Log, len(list))

	for i := 0; i < len(list); i++ {
		logs[i] = &list[i].Log
		var cache = map[string]string{}
		for _, tag := range list[i].Extends {
			cache[*tag.Key] = *tag.Value
		}
		data, _ := json.Marshal(cache)
		logs[i].Contents = []*producer.LogContent{
			{
				LogTimeNs: time.Now().UnixNano(),
				Log:       string(data),
			},
		}

		logs[i].Labels = "{}"
	}
	return logs
}
