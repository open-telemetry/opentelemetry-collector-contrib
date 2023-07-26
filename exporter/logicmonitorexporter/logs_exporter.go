// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"context"
	"fmt"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/logicmonitor/lm-data-sdk-go/utils/translator"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	logs "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/logs"
)

// These are logicmonitor specific constants needed to map the resource with the logs on logicmonitor platform.
const (
	hostname         = "hostname"
	hostnameProperty = "system.hostname"
)

type logExporter struct {
	config   *Config
	sender   *logs.Sender
	settings component.TelemetrySettings
}

// Create new logicmonitor logs exporter
func newLogsExporter(_ context.Context, cfg component.Config, set exporter.CreateSettings) *logExporter {
	oCfg := cfg.(*Config)

	return &logExporter{
		config:   oCfg,
		settings: set.TelemetrySettings,
	}
}

func (e *logExporter) start(ctx context.Context, host component.Host) error {
	client, err := e.config.HTTPClientSettings.ToClient(host, e.settings)
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}

	authParams := utils.AuthParams{
		AccessID:    e.config.APIToken.AccessID,
		AccessKey:   string(e.config.APIToken.AccessKey),
		BearerToken: string(e.config.Headers["Authorization"]),
	}

	e.sender, err = logs.NewSender(ctx, e.config.Endpoint, client, authParams, e.settings.Logger)
	if err != nil {
		return err
	}
	return nil
}

func (e *logExporter) PushLogData(ctx context.Context, lg plog.Logs) error {
	resourceLogs := lg.ResourceLogs()
	var payload []model.LogInput

	for i := 0; i < resourceLogs.Len(); i++ {
		resourceLog := resourceLogs.At(i)
		libraryLogs := resourceLog.ScopeLogs()
		for j := 0; j < libraryLogs.Len(); j++ {
			libraryLog := libraryLogs.At(j)
			logs := libraryLog.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				logMetadataMap := make(map[string]interface{})
				resourceMapperMap := make(map[string]interface{})
				log := logs.At(k)

				log.Attributes().Range(func(key string, value pcommon.Value) bool {
					logMetadataMap[key] = value.AsRaw()
					return true
				})

				resourceLog.Resource().Attributes().Range(func(key string, value pcommon.Value) bool {
					if key == hostname {
						resourceMapperMap[hostnameProperty] = value.AsRaw()
					}
					resourceMapperMap[key] = value.AsRaw()
					return true
				})

				e.settings.Logger.Debug("Sending log data", zap.String("body", log.Body().Str()), zap.Any("resourcemap", resourceMapperMap), zap.Any("metadatamap", logMetadataMap))
				payload = append(payload, translator.ConvertToLMLogInput(log.Body().AsRaw(), timestampFromLogRecord(log).String(), resourceMapperMap, logMetadataMap))
			}
		}
	}
	return e.sender.SendLogs(ctx, payload)
}

func timestampFromLogRecord(lr plog.LogRecord) pcommon.Timestamp {
	if lr.Timestamp() != 0 {
		return lr.Timestamp()
	}

	if lr.ObservedTimestamp() != 0 {
		return lr.ObservedTimestamp()
	}
	return pcommon.NewTimestampFromTime(time.Now())
}
