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
	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

func TestMongoeventToLogData4_4(t *testing.T) {
	mongoevent := GetTestEvent4_4()
	pc := ProjectContext{
		orgName: "Org",
		Project: mongodbatlas.Project{Name: "Project"},
	}

	ld := mongodbEventToLogData(zap.NewNop(), []model.LogEntry{mongoevent}, pc, "hostname", "clusterName", "logName", "4.4")
	rl := ld.ResourceLogs().At(0)
	resourceAttrs := rl.Resource().Attributes()
	sl := rl.ScopeLogs().At(0)
	lr := sl.LogRecords().At(0)
	attrs := lr.Attributes()
	assert.Equal(t, ld.ResourceLogs().Len(), 1)
	assert.Equal(t, resourceAttrs.Len(), 4)
	assert.Equal(t, attrs.Len(), 9)
	assert.Equal(t, pcommon.Timestamp(1663006227215000000), lr.Timestamp())
	_, exists := attrs.Get("id")
	assert.True(t, exists, "expected attribute id to exist, but it didn't")

	// Count attribute will not be present in the LogData
	ld = mongodbEventToLogData(zap.NewNop(), []model.LogEntry{mongoevent}, pc, "hostname", "clusterName", "logName", "4.4")
	assert.Equal(t, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Len(), 9)
}

func TestMongoeventToLogData4_2(t *testing.T) {
	mongoevent := GetTestEvent4_2()
	pc := ProjectContext{
		orgName: "Org",
		Project: mongodbatlas.Project{Name: "Project"},
	}

	ld := mongodbEventToLogData(zaptest.NewLogger(t), []model.LogEntry{mongoevent}, pc, "hostname", "clusterName", "logName", "4.2")
	rl := ld.ResourceLogs().At(0)
	resourceAttrs := rl.Resource().Attributes()
	sl := rl.ScopeLogs().At(0)
	lr := sl.LogRecords().At(0)
	attrs := lr.Attributes()
	assert.Equal(t, ld.ResourceLogs().Len(), 1)
	assert.Equal(t, resourceAttrs.Len(), 4)
	assert.Equal(t, attrs.Len(), 5)
	assert.Equal(t, pcommon.Timestamp(1663004293902000000), lr.Timestamp())
	_, exists := attrs.Get("id")
	assert.False(t, exists, "expected attribute id to not exist, but it did")
}

func TestUnknownSeverity(t *testing.T) {
	mongoevent := GetTestEvent4_4()
	mongoevent.Severity = "Unknown"
	pc := ProjectContext{
		orgName: "Org",
		Project: mongodbatlas.Project{Name: "Project"},
	}

	ld := mongodbEventToLogData(zap.NewNop(), []model.LogEntry{mongoevent}, pc, "hostname", "clusterName", "logName", "4.4")
	rl := ld.ResourceLogs().At(0)
	logEntry := rl.ScopeLogs().At(0).LogRecords().At(0)

	assert.Equal(t, logEntry.SeverityNumber(), plog.SeverityNumberUndefined)
	assert.Equal(t, logEntry.SeverityText(), "")
}

func TestMongoEventToAuditLogData5_0(t *testing.T) {
	mongoevent := GetTestAuditEvent5_0()
	pc := ProjectContext{
		orgName: "Org",
		Project: mongodbatlas.Project{Name: "Project"},
	}

	ld := mongodbAuditEventToLogData(zaptest.NewLogger(t), []model.AuditLog{mongoevent}, pc, "hostname", "logName", "clusterName", "5.0")
	rl := ld.ResourceLogs().At(0)
	resourceAttrs := rl.Resource().Attributes()
	sl := rl.ScopeLogs().At(0)
	lr := sl.LogRecords().At(0)
	attrs := lr.Attributes()

	assert.Equal(t, ld.ResourceLogs().Len(), 1)
	assert.Equal(t, resourceAttrs.Len(), 4)
	assertString(t, resourceAttrs, "mongodb_atlas.org", "Org")
	assertString(t, resourceAttrs, "mongodb_atlas.project", "Project")
	assertString(t, resourceAttrs, "mongodb_atlas.cluster", "clusterName")
	assertString(t, resourceAttrs, "mongodb_atlas.host.name", "hostname")

	assert.Equal(t, 14, attrs.Len())
	assertString(t, attrs, "atype", "authenticate")
	assertString(t, attrs, "local.ip", "0.0.0.0")
	assertInt(t, attrs, "local.port", 3000)
	assertBool(t, attrs, "local.isSystemUser", true)
	assertString(t, attrs, "local.unix", "/var/run/mongodb/mongodb-27017.sock")
	assertString(t, attrs, "remote.ip", "192.168.1.237")
	assertInt(t, attrs, "remote.port", 4000)
	assertString(t, attrs, "uuid.binary", "binary")
	assertString(t, attrs, "uuid.type", "type")
	assertString(t, attrs, "log_name", "logName")
	assertInt(t, attrs, "result", 40)

	roles, ok := attrs.Get("roles")
	require.True(t, ok, "roles key does not exist")
	require.Equal(t, roles.SliceVal().Len(), 1)
	assertString(t, roles.SliceVal().At(0).MapVal(), "role", "test_role")
	assertString(t, roles.SliceVal().At(0).MapVal(), "db", "test_db")

	users, ok := attrs.Get("users")
	require.True(t, ok, "users key does not exist")
	require.Equal(t, users.SliceVal().Len(), 1)
	assertString(t, users.SliceVal().At(0).MapVal(), "user", "mongo_user")
	assertString(t, users.SliceVal().At(0).MapVal(), "db", "my_db")

	param, ok := attrs.Get("param")
	require.True(t, ok, "param key does not exist")
	assert.Equal(t, mongoevent.Param, param.MapVal().AsRaw())

	assert.Equal(t, pcommon.Timestamp(1663342012563000000), lr.Timestamp())
	assert.Equal(t, plog.SeverityNumberInfo, lr.SeverityNumber())
	assert.Equal(t, "INFO", lr.SeverityText())
	assert.Equal(t, `{"atype":"authenticate","ts":{"$date":"2022-09-16T15:26:52.563+00:00"},"uuid":{"$binary":"binary","$type":"type"},"local":{"ip":"0.0.0.0","port":3000,"isSystemUser":true,"unix":"/var/run/mongodb/mongodb-27017.sock"},"remote":{"ip":"192.168.1.237","port":4000},"users":[{"user":"mongo_user","db":"my_db"}],"roles":[{"role":"test_role","db":"test_db"}],"result":40,"param":{"db":"db","mechanism":"mechanism","user":"name"}}`,
		lr.Body().StringVal())
}

func TestMongoEventToAuditLogData4_2(t *testing.T) {
	mongoevent := GetTestAuditEvent4_2()
	pc := ProjectContext{
		orgName: "Org",
		Project: mongodbatlas.Project{Name: "Project"},
	}

	ld := mongodbAuditEventToLogData(zaptest.NewLogger(t), []model.AuditLog{mongoevent}, pc, "hostname", "logName", "clusterName", "4.2")
	rl := ld.ResourceLogs().At(0)
	resourceAttrs := rl.Resource().Attributes()
	sl := rl.ScopeLogs().At(0)
	lr := sl.LogRecords().At(0)
	attrs := lr.Attributes()

	assert.Equal(t, ld.ResourceLogs().Len(), 1)
	assert.Equal(t, resourceAttrs.Len(), 4)
	assertString(t, resourceAttrs, "mongodb_atlas.org", "Org")
	assertString(t, resourceAttrs, "mongodb_atlas.project", "Project")
	assertString(t, resourceAttrs, "mongodb_atlas.cluster", "clusterName")
	assertString(t, resourceAttrs, "mongodb_atlas.host.name", "hostname")

	assert.Equal(t, 10, attrs.Len())
	assertString(t, attrs, "atype", "authenticate")
	assertString(t, attrs, "local.ip", "0.0.0.0")
	assertInt(t, attrs, "local.port", 3000)
	assertString(t, attrs, "remote.ip", "192.168.1.237")
	assertInt(t, attrs, "remote.port", 4000)

	assertString(t, attrs, "log_name", "logName")
	assertInt(t, attrs, "result", 40)

	roles, ok := attrs.Get("roles")
	require.True(t, ok, "roles key does not exist")
	require.Equal(t, roles.SliceVal().Len(), 1)
	assertString(t, roles.SliceVal().At(0).MapVal(), "role", "test_role")
	assertString(t, roles.SliceVal().At(0).MapVal(), "db", "test_db")

	users, ok := attrs.Get("users")
	require.True(t, ok, "users key does not exist")
	require.Equal(t, users.SliceVal().Len(), 1)
	assertString(t, users.SliceVal().At(0).MapVal(), "user", "mongo_user")
	assertString(t, users.SliceVal().At(0).MapVal(), "db", "my_db")

	param, ok := attrs.Get("param")
	require.True(t, ok, "param key does not exist")
	assert.Equal(t, mongoevent.Param, param.MapVal().AsRaw())

	assert.Equal(t, pcommon.Timestamp(1663342012563000000), lr.Timestamp())
	assert.Equal(t, plog.SeverityNumberInfo, lr.SeverityNumber())
	assert.Equal(t, "INFO", lr.SeverityText())
	assert.Equal(t, `{"atype":"authenticate","ts":{"$date":"2022-09-16T15:26:52.563+0000"},"local":{"ip":"0.0.0.0","port":3000},"remote":{"ip":"192.168.1.237","port":4000},"users":[{"user":"mongo_user","db":"my_db"}],"roles":[{"role":"test_role","db":"test_db"}],"result":40,"param":{"db":"db","mechanism":"mechanism","user":"name"}}`,
		lr.Body().StringVal())
}

func GetTestEvent4_4() model.LogEntry {
	return model.LogEntry{
		Timestamp: model.LogTimestamp{
			Date: "2022-09-12T18:10:27.215+00:00",
		},
		Severity:   "I",
		Component:  "NETWORK",
		ID:         12312,
		Context:    "context",
		Message:    "Connection ended",
		Attributes: map[string]interface{}{"connectionCount": 47, "connectionId": 9052, "remote": "192.168.253.105:59742", "id": "93a8f190-afd0-422d-9de6-f6c5e833e35f"},
	}
}

func GetTestEvent4_2() model.LogEntry {
	return model.LogEntry{
		Severity:  "I",
		Component: "NETWORK",
		Context:   "context",
		Message:   "Connection ended",
		Timestamp: model.LogTimestamp{
			Date: "2022-09-12T17:38:13.902+0000",
		},
	}
}

func GetTestAuditEvent5_0() model.AuditLog {
	return model.AuditLog{
		Timestamp: model.LogTimestamp{
			Date: "2022-09-16T15:26:52.563+00:00",
		},
		Type: "authenticate",
		ID: &model.ID{
			Type:   "type",
			Binary: "binary",
		},
		Local: model.Address{
			IP:         strp("0.0.0.0"),
			Port:       intp(3000),
			SystemUser: boolp(true),
			UnixSocket: strp("/var/run/mongodb/mongodb-27017.sock"),
		},
		Remote: model.Address{
			IP:   strp("192.168.1.237"),
			Port: intp(4000),
		},
		Roles: []model.AuditRole{
			{
				Role:     "test_role",
				Database: "test_db",
			},
		},
		Users: []model.AuditUser{
			{
				User:     "mongo_user",
				Database: "my_db",
			},
		},
		Result: 40,
		Param: map[string]any{
			"user":      "name",
			"db":        "db",
			"mechanism": "mechanism",
		},
	}
}

func GetTestAuditEvent4_2() model.AuditLog {
	return model.AuditLog{
		Timestamp: model.LogTimestamp{
			Date: "2022-09-16T15:26:52.563+0000",
		},
		Type: "authenticate",
		Local: model.Address{
			IP:   strp("0.0.0.0"),
			Port: intp(3000),
		},
		Remote: model.Address{
			IP:   strp("192.168.1.237"),
			Port: intp(4000),
		},
		Roles: []model.AuditRole{
			{
				Role:     "test_role",
				Database: "test_db",
			},
		},
		Users: []model.AuditUser{
			{
				User:     "mongo_user",
				Database: "my_db",
			},
		},
		Result: 40,
		Param: map[string]any{
			"user":      "name",
			"db":        "db",
			"mechanism": "mechanism",
		},
	}
}

func assertString(t *testing.T, m pcommon.Map, key, expected string) {
	t.Helper()

	v, ok := m.Get(key)
	if !ok {
		assert.Fail(t, "Couldn't find key %s in map", key)
		return
	}

	if v.Type() != pcommon.ValueTypeString {
		assert.Fail(t, "Value for key %s was expected be STRING but was %s", key, v.Type().String())
	}

	assert.Equal(t, expected, v.StringVal())
}

func assertInt(t *testing.T, m pcommon.Map, key string, expected int64) {
	t.Helper()

	v, ok := m.Get(key)
	if !ok {
		assert.Fail(t, "Couldn't find key %s in map", key)
		return
	}

	if v.Type() != pcommon.ValueTypeInt {
		assert.Fail(t, "Value for key %s was expected be INT but was %s", key, v.Type().String())
	}

	assert.Equal(t, expected, v.IntVal())
}

func assertBool(t *testing.T, m pcommon.Map, key string, expected bool) {
	t.Helper()

	v, ok := m.Get(key)
	if !ok {
		assert.Fail(t, "Couldn't find key %s in map", key)
		return
	}

	if v.Type() != pcommon.ValueTypeBool {
		assert.Fail(t, "Value for key %s was expected be BOOL but was %s", key, v.Type().String())
	}

	assert.Equal(t, expected, v.BoolVal())
}
