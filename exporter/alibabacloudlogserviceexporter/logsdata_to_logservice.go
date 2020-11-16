// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alibabacloudlogserviceexporter

import (
	"encoding/json"
	"strconv"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	slsLogTimeUnixNano   = "timeUnixNano"
	slsLogSeverityNumber = "severityNumber"
	slsLogSeverityText   = "severityText"
	slsLogName           = "name"
	slsLogContent        = "content"
	slsLogAttribute      = "attribute"
	slsLogFlags          = "flags"
)

func logDataToLogService(logger *zap.Logger, ld pdata.Logs) ([]*sls.Log, int) {
	numDroppedLogs := 0
	slsLogs := make([]*sls.Log, 0)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		if rl.IsNil() {
			continue
		}

		ills := rl.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			if ils.IsNil() {
				continue
			}

			logs := ils.Logs()
			for j := 0; j < logs.Len(); j++ {
				lr := logs.At(j)
				if lr.IsNil() {
					continue
				}
				slsLog := mapLogRecordToLogService(lr, logger)
				if slsLog == nil {
					numDroppedLogs++
				} else {
					slsLogs = append(slsLogs, slsLog)
				}
			}
		}
	}

	return slsLogs, numDroppedLogs
}

func mapLogRecordToLogService(lr pdata.LogRecord, logger *zap.Logger) *sls.Log {
	if lr.Body().IsNil() {
		return nil
	}
	var slsLog sls.Log

	// pre alloc, refine if logContent's len > 16
	slsLog.Contents = make([]*sls.LogContent, 0, 16)

	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(slsLogTimeUnixNano),
		Value: proto.String(strconv.FormatUint(uint64(lr.Timestamp()), 10)),
	})

	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(slsLogSeverityNumber),
		Value: proto.String(strconv.FormatInt(int64(lr.SeverityNumber()), 10)),
	})

	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(slsLogSeverityText),
		Value: proto.String(lr.SeverityText()),
	})

	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(slsLogName),
		Value: proto.String(lr.Name()),
	})

	fields := map[string]interface{}{}
	lr.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		fields[k] = convertAttributeValue(v, logger)
	})
	attributeBuffer, _ := json.Marshal(fields)
	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(slsLogAttribute),
		Value: proto.String(string(attributeBuffer)),
	})

	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(slsLogContent),
		Value: proto.String(convertAttributeValueToString(lr.Body(), logger)),
	})

	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(slsLogFlags),
		Value: proto.String(strconv.FormatUint(uint64(lr.Flags()), 16)),
	})

	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(traceIDField),
		Value: proto.String(lr.TraceID().HexString()),
	})

	slsLog.Contents = append(slsLog.Contents, &sls.LogContent{
		Key:   proto.String(spanIDField),
		Value: proto.String(lr.SpanID().HexString()),
	})

	if lr.Timestamp() > 0 {
		// convert time nano to time seconds
		slsLog.Time = proto.Uint32(uint32(lr.Timestamp() / 1000000000))
	} else {
		slsLog.Time = proto.Uint32(uint32(time.Now().Unix()))
	}

	return &slsLog
}

func convertAttributeValueToString(value pdata.AttributeValue, logger *zap.Logger) string {
	switch value.Type() {
	case pdata.AttributeValueINT:
		return strconv.FormatInt(value.IntVal(), 10)
	case pdata.AttributeValueBOOL:
		return strconv.FormatBool(value.BoolVal())
	case pdata.AttributeValueDOUBLE:
		return strconv.FormatFloat(value.DoubleVal(), 'g', -1, 64)
	case pdata.AttributeValueSTRING:
		return value.StringVal()
	case pdata.AttributeValueMAP:
		values := map[string]interface{}{}
		value.MapVal().ForEach(func(k string, v pdata.AttributeValue) {
			values[k] = convertAttributeValue(v, logger)
		})
		buffer, _ := json.Marshal(values)
		return string(buffer)
	case pdata.AttributeValueARRAY:
		arrayVal := value.ArrayVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = convertAttributeValue(arrayVal.At(i), logger)
		}
		buffer, _ := json.Marshal(values)
		return string(buffer)
	case pdata.AttributeValueNULL:
		return ""
	default:
		logger.Debug("Unhandled value type", zap.String("type", value.Type().String()))
		return ""
	}
}

func convertAttributeValue(value pdata.AttributeValue, logger *zap.Logger) interface{} {
	switch value.Type() {
	case pdata.AttributeValueINT:
		return value.IntVal()
	case pdata.AttributeValueBOOL:
		return value.BoolVal()
	case pdata.AttributeValueDOUBLE:
		return value.DoubleVal()
	case pdata.AttributeValueSTRING:
		return value.StringVal()
	case pdata.AttributeValueMAP:
		values := map[string]interface{}{}
		value.MapVal().ForEach(func(k string, v pdata.AttributeValue) {
			values[k] = convertAttributeValue(v, logger)
		})
		return values
	case pdata.AttributeValueARRAY:
		arrayVal := value.ArrayVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = convertAttributeValue(arrayVal.At(i), logger)
		}
		return values
	case pdata.AttributeValueNULL:
		return nil
	default:
		logger.Debug("Unhandled value type", zap.String("type", value.Type().String()))
		return value
	}
}
