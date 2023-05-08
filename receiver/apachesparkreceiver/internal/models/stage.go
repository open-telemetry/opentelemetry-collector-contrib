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
	NumActiveTasks               int64  `json:"numActiveTasks"`
	NumCompleteTasks             int64  `json:"numCompleteTasks"`
	NumFailedTasks               int64  `json:"numFailedTasks"`
	NumKilledTasks               int64  `json:"numKilledTasks"`
	ExecutorRunTime              int64  `json:"executorRunTime"`
	ExecutorCPUTime              int64  `json:"executorCpuTime"`
	ResultSize                   int64  `json:"resultSize"`
	JvmGcTime                    int64  `json:"jvmGcTime"`
	MemoryBytesSpilled           int64  `json:"memoryBytesSpilled"`
	DiskBytesSpilled             int64  `json:"diskBytesSpilled"`
	PeakExecutionMemory          int64  `json:"peakExecutionMemory"`
	InputBytes                   int64  `json:"inputBytes"`
	InputRecords                 int64  `json:"inputRecords"`
	OutputBytes                  int64  `json:"outputBytes"`
	OutputRecords                int64  `json:"outputRecords"`
	ShuffleRemoteBlocksFetched   int64  `json:"shuffleRemoteBlocksFetched"`
	ShuffleLocalBlocksFetched    int64  `json:"shuffleLocalBlocksFetched"`
	ShuffleFetchWaitTime         int64  `json:"shuffleFetchWaitTime"`
	ShuffleRemoteBytesRead       int64  `json:"shuffleRemoteBytesRead"`
	ShuffleLocalBytesRead        int64  `json:"shuffleLocalBytesRead"`
	ShuffleRemoteBytesReadToDisk int64  `json:"shuffleRemoteBytesReadToDisk"`
	ShuffleReadBytes             int64  `json:"shuffleReadBytes"`
	ShuffleReadRecords           int64  `json:"shuffleReadRecords"`
	ShuffleWriteBytes            int64  `json:"shuffleWriteBytes"`
	ShuffleWriteRecords          int64  `json:"shuffleWriteRecords"`
	ShuffleWriteTime             int64  `json:"shuffleWriteTime"`
}
