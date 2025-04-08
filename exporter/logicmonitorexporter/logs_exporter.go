// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"context"
	"fmt"
	"net/http"
	"time"

	lmsdklogs "github.com/logicmonitor/lm-data-sdk-go/api/logs"
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
	cancel   context.CancelFunc
}

// Create new logicmonitor logs exporter
func newLogsExporter(_ context.Context, cfg component.Config, set exporter.Settings) *logExporter {
	oCfg := cfg.(*Config)

	return &logExporter{
		config:   oCfg,
		settings: set.TelemetrySettings,
	}
}

func (e *logExporter) start(ctx context.Context, host component.Host) error {
	client, err := e.config.ToClient(ctx, host, e.settings)
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}

	opts := buildLogIngestOpts(e.config, client)

	ctx, e.cancel = context.WithCancel(ctx)
	e.sender, err = logs.NewSender(ctx, e.settings.Logger, opts...)
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
				logMetadataMap := make(map[string]any)
				resourceMapperMap := make(map[string]any)
				log := logs.At(k)

				for key, value := range log.Attributes().All() {
					logMetadataMap[key] = value.AsRaw()
				}

				for key, value := range resourceLog.Resource().Attributes().All() {
					if key == hostname {
						resourceMapperMap[hostnameProperty] = value.AsRaw()
					}
					resourceMapperMap[key] = value.AsRaw()
				}

				e.settings.Logger.Debug("Sending log data", zap.String("body", log.Body().Str()), zap.Any("resourcemap", resourceMapperMap), zap.Any("metadatamap", logMetadataMap))
				payload = append(payload, translator.ConvertToLMLogInput(log.Body().AsRaw(), timestampFromLogRecord(log).String(), resourceMapperMap, logMetadataMap))
			}
		}
	}
	return e.sender.SendLogs(ctx, payload)
}

func (e *logExporter) shutdown(_ context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}

	return nil
}

func buildLogIngestOpts(config *Config, client *http.Client) []lmsdklogs.Option {
	authParams := utils.AuthParams{
		AccessID:    config.APIToken.AccessID,
		AccessKey:   string(config.APIToken.AccessKey),
		BearerToken: string(config.Headers["Authorization"]),
	}

	opts := []lmsdklogs.Option{
		lmsdklogs.WithLogBatchingDisabled(),
		lmsdklogs.WithAuthentication(authParams),
		lmsdklogs.WithHTTPClient(client),
		lmsdklogs.WithEndpoint(config.Endpoint),
	}

	if config.Logs.ResourceMappingOperation != "" {
		opts = append(opts, lmsdklogs.WithResourceMappingOperation(string(config.Logs.ResourceMappingOperation)))
	}

	return opts
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
