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

package observiqexporter

import "go.opentelemetry.io/collector/model/pdata"

type observIQLogBatch struct {
	Logs []*observIQLog `json:"logs"`
}

type observIQLog struct {
	ID    string           `json:"id"`
	Size  int              `json:"size"`
	Entry observIQLogEntry `json:"entry"`
}

type observIQAgentInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type observIQLogEntry struct {
	Timestamp string                 `json:"@timestamp"`
	Severity  string                 `json:"severity,omitempty"`
	EntryType string                 `json:"type,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Resource  interface{}            `json:"resource,omitempty"`
	Agent     *observIQAgentInfo     `json:"agent,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Body      map[string]interface{} `json:"body,omitempty"`
}

// Convert pdata.Logs to observIQLogBatch
func logdataToObservIQFormat(ld pdata.Logs, agentID string, agentName string) (*observIQLogBatch, []error) {
	return &observIQLogBatch{}, []error{}
}
