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

// logTimestamp is the structure that represents a Log Timestamp
type logTimestamp struct {
	Date string `json:"$date"`
}

// {"t":{"$date":"2022-07-12T16:44:55.709+00:00"},"s":"I",  "c":"NETWORK",  "id":22943,   "ctx":"listener","msg":"Connection accepted","attr":{"remote":"192.168.254.61:58838","uuid":"dc95f0c0-bd0f-4266-a120-2248362a70db","connectionId":313919,"connectionCount":35}}
