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

package groupbyauthprocessor

import (
	"go.opentelemetry.io/collector/consumer/pdata"
)

// storage is an abstraction for the span storage used by the groupbyauth processor.
// Implementations should be safe for concurrent use.
type storage interface {
	// createOrAppend will check whether the given token is already in the storage and
	// will either append the given spans to the existing record, or create a new pdate.Traces with
	// the given content
	createOrAppend(string, pdata.Traces) error

	// get will retrieve the trace based on the given token, returning false in case a trace
	// cannot be found
	get(string) (pdata.Traces, bool)

	// delete will remove the trace based on the given token, returning the trace that was removed,
	// or empty pdata.Traces and false in case a trace cannot be found
	delete(string) (pdata.Traces, bool)

	// start gives the storage the opportunity to initialize any resources or procedures
	start() error

	// shutdown signals the storage that the processor is shutting down
	shutdown() error
}
