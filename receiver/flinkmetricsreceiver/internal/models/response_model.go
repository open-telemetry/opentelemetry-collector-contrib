// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
