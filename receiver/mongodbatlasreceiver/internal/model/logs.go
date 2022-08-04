// Copyright  The OpenTelemetry Authors
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

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"

// LogEntry represents a MongoDB Atlas JSON log entry
type LogEntry struct {
	Timestamp  logTimestamp           `json:"t"`
	Severity   string                 `json:"s"`
	Component  string                 `json:"c"`
	ID         int64                  `json:"id"`
	Context    string                 `json:"ctx"`
	Message    string                 `json:"msg"`
	Attributes map[string]interface{} `json:"attr"`
}

// AuditLog represents a MongoDB Atlas JSON audit log entry
type AuditLog struct {
	AuthType  string       `json:"authenticate"`
	Timestamp logTimestamp `json:"ts"`
	ID        id           `json:"uuid"`
	Local     address      `json:"local"`
	Remote    address      `json:"remote"`
	Result    int          `json:"result"`
	Param     param        `json:"param"`
}

// logTimestamp is the structure that represents a Log Timestamp
type logTimestamp struct {
	Date string `json:"$date"`
}

type id struct {
	Binary string `json:"$binary"`
	Type   string `json:"$type"`
}

type address struct {
	IP   string `json:"Ip"`
	Port int    `json:"port"`
}

type param struct {
	User      string `json:"user"`
	Database  string `json:"db"`
	Mechanism string `json:"mechanism"`
}

func GetTestEvent() LogEntry {
	return LogEntry{
		Severity:   "I",
		Component:  "NETWORK",
		ID:         12312,
		Context:    "context",
		Message:    "Connection ended",
		Attributes: map[string]interface{}{"connectionCount": 47, "connectionId": 9052, "remote": "192.168.253.105:59742", "id": "93a8f190-afd0-422d-9de6-f6c5e833e35f"},
	}
}

func GetTestAuditEvent() AuditLog {
	return AuditLog{
		AuthType: "authtype",
		ID: id{
			Type:   "type",
			Binary: "binary",
		},
		Local: address{
			IP:   "Ip",
			Port: 12345,
		},
		Remote: address{
			IP:   "Ip",
			Port: 12345,
		},
		Result: 40,
		Param: param{
			User:      "name",
			Database:  "db",
			Mechanism: "mechanism",
		},
	}
}
