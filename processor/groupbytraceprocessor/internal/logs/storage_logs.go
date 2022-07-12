// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/logs"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// storage is an abstraction for the span storage used by the groupbytrace log processor.
// Implementations should be safe for concurrent use.
type Storage interface {
	// createOrAppend will check whether the given  log ID is already in the storage and
	// will either append the given logRecords to the existing record, or create a new  log with
	// the given logRecords from log
	createOrAppend(pcommon.TraceID, plog.Logs) error

	// get will retrieve the  log based on the given  log ID, returning nil in case a log
	// cannot be found
	get(pcommon.TraceID) ([]plog.ResourceLogs, error)

	// delete will remove the  log based on the given  log ID, returning the  log that was removed,
	// or nil in case a  log cannot be found
	delete(pcommon.TraceID) ([]plog.ResourceLogs, error)

	// start gives the storage the opportunity to initialize any resources or procedures
	start() error

	// shutdown signals the storage that the processor is shutting down
	shutdown() error
}
