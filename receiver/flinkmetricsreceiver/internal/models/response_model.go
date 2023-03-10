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

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/models"

// MetricsResponse stores a response for the metric endpoints.
type MetricsResponse []struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// TaskmanagerIDsResponse stores a response for the taskmanagers endpoint.
type TaskmanagerIDsResponse struct {
	Taskmanagers []struct {
		ID string `json:"id"`
	} `json:"taskmanagers"`
}

// JobOverviewResponse stores a response for the jobs overview endpoint.
type JobOverviewResponse struct {
	Jobs []struct {
		Jid  string `json:"jid"`
		Name string `json:"name"`
	} `json:"jobs"`
}

// JobsResponse stores a response for the jobs endpoint.
type JobsResponse struct {
	Jobs []struct {
		ID string `json:"id"`
	} `json:"jobs"`
}

// JobsWithIDResponse stores a response for the jobs with ID endpoint.
type JobsWithIDResponse struct {
	Jid      string `json:"jid"`
	Name     string `json:"name"`
	Vertices []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"vertices"`
}

// VerticesResponse stores a response for the vertices endpoint.
type VerticesResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subtasks []struct {
		Subtask       int    `json:"subtask"`
		Host          string `json:"host"`
		TaskmanagerID string `json:"taskmanager-id"`
	} `json:"subtasks"`
}
