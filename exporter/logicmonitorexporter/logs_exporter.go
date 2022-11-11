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

	"github.com/logicmonitor/lm-data-sdk-go/api/logs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logExporter struct {
	config          *Config
	logger          *zap.Logger
	logIngestClient *logs.LMLogIngest
}

const (
	hostname         = "hostname"
	hostnameProperty = "system.hostname"
)

// Create new logs exporter
func newLogsExporter(cfg component.ExporterConfig, logger *zap.Logger) (*logExporter, error) {
	oCfg := cfg.(*Config)

	isBatchEnabled := oCfg.LogBatchingEnabled
	batchInterval := oCfg.LogBatchingInterval

	if batchInterval < MinLogBatchInterval {
		return nil, fmt.Errorf("Minimum batching interval should be " + MinLogBatchInterval.String())
	}

	var options []logs.Option
	if isBatchEnabled {
		options = []logs.Option{
			logs.WithLogBatchingInterval(batchInterval),
		}
	}

	auth := LMAuthenticator{
		Config: oCfg,
	}
	options = append(options, logs.WithAuthentication(auth))

	lli, err := logs.NewLMLogIngest(context.Background(), options...)
	if err != nil {
		return nil, err
	}
	return &logExporter{
		config:          oCfg,
		logger:          logger,
		logIngestClient: lli,
	}, nil
}

func (e *logExporter) PushLogData(ctx context.Context, lg plog.Logs) (er error) {
	resourceLogs := lg.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resourceLog := resourceLogs.At(i)
		libraryLogs := resourceLog.ScopeLogs()
		for j := 0; j < libraryLogs.Len(); j++ {
			libraryLog := libraryLogs.At(j)
			logs := libraryLog.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				logMetadataMap := make(map[string]string)
				resourceMapperMap := make(map[string]string)
				log := logs.At(k)
				// Copying resource attributes to log attributes
				resourceLog.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
					logAttributes := log.Attributes()
					e.mergeAttributes(k, v, logAttributes)
					return true
				})
				attributesMap := log.Attributes()
				attributesMap.Range(func(key string, value pcommon.Value) bool {
					if key == hostname {
						resourceMapperMap[hostnameProperty] = value.Str()
					}
					logMetadataMap[key] = value.Str()
					return true
				})
				err := e.logIngestClient.SendLogs(ctx, log.Body().Str(), resourceMapperMap, logMetadataMap)
				if err != nil {
					e.logger.Error("error while sending logs ", zap.Error(err))
				}
			}
		}
	}
	return nil
}

func (e *logExporter) mergeAttributes(k string, value pcommon.Value, logAttr pcommon.Map) {
	switch value.Type() {
	case pcommon.ValueTypeInt:
		logAttr.PutInt(k, value.Int())
	case pcommon.ValueTypeBool:
		logAttr.PutBool(k, value.Bool())
	case pcommon.ValueTypeDouble:
		logAttr.PutDouble(k, value.Double())
	case pcommon.ValueTypeStr:
		logAttr.PutStr(k, value.Str())
	case pcommon.ValueTypeMap:
		values := map[string]interface{}{}
		value.Map().Range(func(k string, v pcommon.Value) bool {
			values[k] = v
			return true
		})
	case pcommon.ValueTypeSlice:
		arrayVal := value.Slice()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = arrayVal.At(i)
		}
	default:
		e.logger.Debug("Unhandled value type", zap.String("type", value.Type().String()))
	}
}
