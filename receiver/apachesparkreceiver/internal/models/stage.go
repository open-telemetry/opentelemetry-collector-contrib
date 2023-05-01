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

// Stages represents the top level json returned by the api/v1/applications/[app-id]/stages endpoint
type Stages []struct {
	Status                       string `json:"status"`
	StageID                      int64  `json:"stageId"`
	AttemptID                    int64  `json:"attemptId"`
	NumActiveTasks               int    `json:"numActiveTasks"`
	NumCompleteTasks             int    `json:"numCompleteTasks"`
	NumFailedTasks               int    `json:"numFailedTasks"`
	NumKilledTasks               int    `json:"numKilledTasks"`
	ExecutorRunTime              int    `json:"executorRunTime"`
	ExecutorCPUTime              int    `json:"executorCpuTime"`
	ResultSize                   int    `json:"resultSize"`
	JvmGcTime                    int    `json:"jvmGcTime"`
	MemoryBytesSpilled           int    `json:"memoryBytesSpilled"`
	DiskBytesSpilled             int    `json:"diskBytesSpilled"`
	PeakExecutionMemory          int    `json:"peakExecutionMemory"`
	InputBytes                   int    `json:"inputBytes"`
	InputRecords                 int    `json:"inputRecords"`
	OutputBytes                  int    `json:"outputBytes"`
	OutputRecords                int    `json:"outputRecords"`
	ShuffleRemoteBlocksFetched   int    `json:"shuffleRemoteBlocksFetched"`
	ShuffleLocalBlocksFetched    int    `json:"shuffleLocalBlocksFetched"`
	ShuffleFetchWaitTime         int    `json:"shuffleFetchWaitTime"`
	ShuffleRemoteBytesRead       int    `json:"shuffleRemoteBytesRead"`
	ShuffleLocalBytesRead        int    `json:"shuffleLocalBytesRead"`
	ShuffleRemoteBytesReadToDisk int    `json:"shuffleRemoteBytesReadToDisk"`
	ShuffleReadBytes             int    `json:"shuffleReadBytes"`
	ShuffleReadRecords           int    `json:"shuffleReadRecords"`
	ShuffleWriteBytes            int    `json:"shuffleWriteBytes"`
	ShuffleWriteRecords          int    `json:"shuffleWriteRecords"`
	ShuffleWriteTime             int    `json:"shuffleWriteTime"`
}
