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

// Jobs represents the top level json returned by the api/v1/applications/[app-id]/jobs endpoint
type Jobs []struct {
	JobId              int64 `json:"jobId"`
	NumActiveTasks     int64 `json:"numActiveTasks"`
	NumCompletedTasks  int64 `json:"numCompletedTasks"`
	NumSkippedTasks    int64 `json:"numSkippedTasks"`
	NumFailedTasks     int64 `json:"numFailedTasks"`
	NumActiveStages    int64 `json:"numActiveStages"`
	NumCompletedStages int64 `json:"numCompletedStages"`
	NumSkippedStages   int64 `json:"numSkippedStages"`
	NumFailedStages    int64 `json:"numFailedStages"`
}
