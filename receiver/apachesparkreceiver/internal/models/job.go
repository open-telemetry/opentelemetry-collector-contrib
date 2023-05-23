// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"

// Jobs represents the top level json returned by the api/v1/applications/[app-id]/jobs endpoint
type Job struct {
	JobID              int64 `json:"jobId"`
	NumActiveTasks     int64 `json:"numActiveTasks"`
	NumCompletedTasks  int64 `json:"numCompletedTasks"`
	NumSkippedTasks    int64 `json:"numSkippedTasks"`
	NumFailedTasks     int64 `json:"numFailedTasks"`
	NumActiveStages    int64 `json:"numActiveStages"`
	NumCompletedStages int64 `json:"numCompletedStages"`
	NumSkippedStages   int64 `json:"numSkippedStages"`
	NumFailedStages    int64 `json:"numFailedStages"`
}
