// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"

// Executors represents the top level json returned by the api/v1/applications/[app-id]/executors endpoint
type Executor struct {
	ExecutorID                string `json:"id"`
	MemoryUsed                int64  `json:"memoryUsed"`
	DiskUsed                  int64  `json:"diskUsed"`
	MaxTasks                  int64  `json:"maxTasks"`
	ActiveTasks               int64  `json:"activeTasks"`
	FailedTasks               int64  `json:"failedTasks"`
	CompletedTasks            int64  `json:"completedTasks"`
	TotalDuration             int64  `json:"totalDuration"`
	TotalGCTime               int64  `json:"totalGCTime"`
	TotalInputBytes           int64  `json:"totalInputBytes"`
	TotalShuffleRead          int64  `json:"totalShuffleRead"`
	TotalShuffleWrite         int64  `json:"totalShuffleWrite"`
	UsedOnHeapStorageMemory   int64  `json:"usedOnHeapStorageMemory"`
	UsedOffHeapStorageMemory  int64  `json:"usedOffHeapStorageMemory"`
	TotalOnHeapStorageMemory  int64  `json:"totalOnHeapStorageMemory"`
	TotalOffHeapStorageMemory int64  `json:"totalOffHeapStorageMemory"`
}
