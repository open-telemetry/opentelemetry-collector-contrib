// Copyright  OpenTelemetry Authors
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

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"encoding/json"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

const (
	// Number of log attributes to add to the plog.LogRecordSlice.
	totalLogAttributes = 5

	// Number of resource attributes to add to the plog.ResourceLogs.
	totalResourceAttributes = 4
)

// layout for the timestamp format in the plog.Logs structure
const layout = "2006-01-02T15:04:05.000-07:00"

// Severity mapping of the mongodb atlas logs
var severityMap = map[string]plog.SeverityNumber{
	"F":  plog.SeverityNumberFATAL,
	"E":  plog.SeverityNumberERROR,
	"W":  plog.SeverityNumberWARN,
	"I":  plog.SeverityNumberINFO,
	"D":  plog.SeverityNumberDEBUG,
	"D1": plog.SeverityNumberDEBUG,
	"D2": plog.SeverityNumberDEBUG2,
	"D3": plog.SeverityNumberDEBUG3,
	"D4": plog.SeverityNumberDEBUG4,
	"D5": plog.SeverityNumberDEBUG4,
}

// mongoAuditEventToLogRecord converts model.AuditLog event to plog.LogRecordSlice and adds the resource attributes.
func mongodbAuditEventToLogData(logger *zap.Logger, e *model.AuditLog, r resourceInfo) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.EnsureCapacity(totalResourceAttributes)

	// Attributes related to the object causing the event.
	resourceAttrs.InsertString("org", r.Org.Name)
	resourceAttrs.InsertString("project", r.Project.Name)
	resourceAttrs.InsertString("cluster", r.Cluster.Name)
	resourceAttrs.InsertString("hostname", r.Hostname)

	data, err := json.Marshal(e)
	if err != nil {
		logger.Warn("failed to marshal", zap.Error(err))
	}

	t, err := time.Parse(layout, e.Timestamp.Date)
	if err != nil {
		logger.Warn("Time failed to parse correctly", zap.Error(err))
	}
	lr.SetTimestamp(pcommon.NewTimestampFromTime(t))
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Insert Raw Log message into Body of LogRecord
	lr.Body().SetStringVal(string(data))

	// Since Audit Logs don't have a severity/level
	// Set the "SeverityNumber" and "SeverityText" to INFO
	lr.SetSeverityNumber(plog.SeverityNumberINFO)
	lr.SetSeverityText("INFO")

	attrs := lr.Attributes()
	attrs.EnsureCapacity(totalLogAttributes)

	if e.AuthType != "" {
		attrs.InsertString("authtype", e.AuthType)
	}

	attrs.InsertString("local.ip", e.Local.Ip)
	attrs.InsertInt("local.port", int64(e.Local.Port))
	attrs.InsertString("remote.ip", e.Remote.Ip)
	attrs.InsertInt("remote.port", int64(e.Remote.Port))
	attrs.InsertString("uuid.binary", e.ID.Binary)
	attrs.InsertString("uuid.type", e.ID.Type)
	attrs.InsertInt("result", int64(e.Result))
	attrs.InsertString("log_name", r.LogName)

	if e.Param.User != "" {
		attrs.InsertString("param.user", e.Param.User)
		attrs.InsertString("param.database", e.Param.Database)
		attrs.InsertString("param.mechanism", e.Param.Mechanism)
	}

	return ld
}

// mongoEventToLogRecord converts model.LogEntry event to plog.LogRecordSlice and adds the resource attributes.
func mongodbEventToLogData(logger *zap.Logger, e *model.LogEntry, r resourceInfo) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.EnsureCapacity(totalResourceAttributes)

	// Attributes related to the object causing the event.
	resourceAttrs.InsertString("org", r.Org.Name)
	resourceAttrs.InsertString("project", r.Project.Name)
	resourceAttrs.InsertString("cluster", r.Cluster.Name)
	resourceAttrs.InsertString("hostname", r.Hostname)

	data, err := json.Marshal(e)
	if err != nil {
		logger.Warn("failed to marshal", zap.Error(err))
	}

	t, err := time.Parse(layout, e.Timestamp.Date)
	if err != nil {
		logger.Warn("Time failed to parse correctly", zap.Error(err))
	}
	lr.SetTimestamp(pcommon.NewTimestampFromTime(t))
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Insert Raw Log message into Body of LogRecord
	lr.Body().SetStringVal(string(data))

	// Set the "SeverityNumber" and "SeverityText" if a known type of
	// severity is found.
	if severityNumber, ok := severityMap[e.Severity]; ok {
		lr.SetSeverityNumber(severityNumber)
		lr.SetSeverityText(e.Severity)
	} else {
		logger.Debug("unknown severity type", zap.String("type", e.Severity))
	}

	attrs := lr.Attributes()
	attrs.EnsureCapacity(totalLogAttributes)

	pcommon.NewMapFromRaw(e.Attributes).CopyTo(attrs)
	attrs.InsertString("message", e.Message)
	attrs.InsertString("component", e.Component)
	attrs.InsertString("context", e.Context)
	attrs.InsertInt("id", e.ID)
	attrs.InsertString("log_name", r.LogName)
	attrs.InsertString("raw", string(data))

	return ld
}
