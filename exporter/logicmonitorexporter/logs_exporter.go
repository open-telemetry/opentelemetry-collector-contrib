// Copyright The OpenTelemetry Authors
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

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"context"
	"fmt"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/api/logs"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/logicmonitor/lm-data-sdk-go/utils/translator"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// These are logicmonitor specific constants needed to map the resource with the logs on logicmonitor platform.
const (
	hostname         = "hostname"
	hostnameProperty = "system.hostname"
)

type logExporter struct {
	config          *Config
	logIngestClient *logs.LMLogIngest
	settings        component.TelemetrySettings
}

// Create new logicmonitor logs exporter
func newLogsExporter(cfg component.Config, set exporter.CreateSettings) *logExporter {
	oCfg := cfg.(*Config)

	return &logExporter{
		config:   oCfg,
		settings: set.TelemetrySettings,
	}
}

func (e *logExporter) start(_ context.Context, host component.Host) error {
	client, err := e.config.HTTPClientSettings.ToClient(host, e.settings)
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}

	authParams := utils.AuthParams{
		AccessID:    e.config.APIToken.AccessID,
		AccessKey:   string(e.config.APIToken.AccessKey),
		BearerToken: string(e.config.Headers["Authorization"]),
	}
	options := []logs.Option{
		logs.WithLogBatchingDisabled(),
		logs.WithAuthentication(authParams),
		logs.WithHTTPClient(client),
		logs.WithEndpoint(e.config.Endpoint),
	}

	e.logIngestClient, err = logs.NewLMLogIngest(context.Background(), options...)
	if err != nil {
		return fmt.Errorf("failed to initialize LMLogIngest: %w", err)
	}
	return nil
}

func (e *logExporter) PushLogData(ctx context.Context, lg plog.Logs) error {
	resourceLogs := lg.ResourceLogs()
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
				payload := translator.ConvertToLMLogInput(log.Body().Str(), timestampFromLogRecord(log).String(), resourceMapperMap, logMetadataMap)
				err := e.logIngestClient.SendLogs(ctx, payload)
				if err != nil {
					e.settings.Logger.Error("error while exporting logs ", zap.Error(err), zap.String("body", log.Body().Str()), zap.Any("resourcemap", resourceMapperMap), zap.Any("metadatamap", logMetadataMap))
					continue
				}
			}
		}
	}
	return nil
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
