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

package mongodbatlasreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

func TestMongoeventToLogData(t *testing.T) {
	mongoevent := GetTestEvent()
	pc := ProjectContext{
		orgName: "Org",
		Project: mongodbatlas.Project{Name: "Project"},
	}

	ld := mongodbEventToLogData(zap.NewNop(), []model.LogEntry{mongoevent}, pc, "hostname", "clusterName", "logName")
	rl := ld.ResourceLogs().At(0)
	resourceAttrs := rl.Resource().Attributes()
	lr := rl.ScopeLogs().At(0)
	attrs := lr.LogRecords().At(0).Attributes()
	assert.Equal(t, ld.ResourceLogs().Len(), 1)
	assert.Equal(t, resourceAttrs.Len(), 4)
	assert.Equal(t, attrs.Len(), 9)

	// Count attribute will not be present in the LogData
	ld = mongodbEventToLogData(zap.NewNop(), []model.LogEntry{mongoevent}, pc, "hostname", "clusterName", "logName")
	assert.Equal(t, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Len(), 9)
}

func TestUnknownSeverity(t *testing.T) {
	mongoevent := GetTestEvent()
	mongoevent.Severity = "Unknown"
	pc := ProjectContext{
		orgName: "Org",
		Project: mongodbatlas.Project{Name: "Project"},
	}

	ld := mongodbEventToLogData(zap.NewNop(), []model.LogEntry{mongoevent}, pc, "hostname", "clusterName", "logName")
	rl := ld.ResourceLogs().At(0)
	logEntry := rl.ScopeLogs().At(0).LogRecords().At(0)

	assert.Equal(t, logEntry.SeverityNumber(), plog.SeverityNumberUndefined)
	assert.Equal(t, logEntry.SeverityText(), "")
}

func TestMongoEventToAuditLogData(t *testing.T) {
	mongoevent := GetTestAuditEvent()
	pc := ProjectContext{
		orgName: "Org",
		Project: mongodbatlas.Project{Name: "Project"},
	}

	ld := mongodbAuditEventToLogData(zap.NewNop(), []model.AuditLog{mongoevent}, pc, "hostname", "clusterName", "logName")
	rl := ld.ResourceLogs().At(0)
	resourceAttrs := rl.Resource().Attributes()
	lr := rl.ScopeLogs().At(0)
	attrs := lr.LogRecords().At(0).Attributes()
	assert.Equal(t, ld.ResourceLogs().Len(), 1)
	assert.Equal(t, resourceAttrs.Len(), 4)
	assert.Equal(t, 12, attrs.Len())

	ld = mongodbAuditEventToLogData(zap.NewNop(), []model.AuditLog{mongoevent}, pc, "hostname", "clusterName", "logName")
	assert.Equal(t, 12, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Len())
}

func GetTestEvent() model.LogEntry {
	return model.LogEntry{
		Severity:   "I",
		Component:  "NETWORK",
		ID:         12312,
		Context:    "context",
		Message:    "Connection ended",
		Attributes: map[string]interface{}{"connectionCount": 47, "connectionId": 9052, "remote": "192.168.253.105:59742", "id": "93a8f190-afd0-422d-9de6-f6c5e833e35f"},
	}
}

func GetTestAuditEvent() model.AuditLog {
	return model.AuditLog{
		AuthType: "authtype",
		ID: model.ID{
			Type:   "type",
			Binary: "binary",
		},
		Local: model.Address{
			IP:   "Ip",
			Port: 12345,
		},
		Remote: model.Address{
			IP:   "Ip",
			Port: 12345,
		},
		Result: 40,
		Param: model.Param{
			User:      "name",
			Database:  "db",
			Mechanism: "mechanism",
		},
	}
}
