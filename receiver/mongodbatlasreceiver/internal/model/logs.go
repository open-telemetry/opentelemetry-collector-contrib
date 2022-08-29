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
	Timestamp  LogTimestamp           `json:"t"`
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
	Timestamp LogTimestamp `json:"ts"`
	ID        ID           `json:"uuid"`
	Local     Address      `json:"local"`
	Remote    Address      `json:"remote"`
	Result    int          `json:"result"`
	Param     Param        `json:"param"`
}

// logTimestamp is the structure that represents a Log Timestamp
type LogTimestamp struct {
	Date string `json:"$date"`
}

type ID struct {
	Binary string `json:"$binary"`
	Type   string `json:"$type"`
}

type Address struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type Param struct {
	User      string `json:"user"`
	Database  string `json:"db"`
	Mechanism string `json:"mechanism"`
}
