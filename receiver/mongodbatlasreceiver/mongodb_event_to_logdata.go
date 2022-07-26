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
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

const (
	// Number of log attributes to add to the plog.LogRecordSlice.
	totalLogAttributes = 5

	// Number of resource attributes to add to the plog.ResourceLogs.
	totalResourceAttributes = 4
)

const layout = "2006-01-02T15:04:05.000-07:00"

// Only two types of events are created as of now.
// For more info: https://docs.openshift.com/container-platform/4.9/rest_api/metadata_apis/event-core-v1.html
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

// k8sEventToLogRecord converts Kubernetes event to plog.LogRecordSlice and adds the resource attributes.
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

	t, err := time.Parse(layout, e.Timestamp.Date)
	if err != nil {
		logger.Warn("Time failed to parse correctly", zap.Error(err))
	}
	lr.SetTimestamp(pcommon.NewTimestampFromTime(t))
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// The Message field contains description about the event,
	// which is best suited for the "Body" of the LogRecordSlice.
	lr.Body().SetStringVal(e.Message)

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

	return ld
}
