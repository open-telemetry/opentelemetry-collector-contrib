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

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"

// Executors represents the top level json returned by the api/v1/applications/[app-id]/executors endpoint
type Executors []struct {
	Id                        string `json:"id"`
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
