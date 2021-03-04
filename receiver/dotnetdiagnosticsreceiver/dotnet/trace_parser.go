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

package dotnet

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

// parseTraceMessage parses a trace message, populating an eventSource struct. The
// results are currently not used, but parsing this message is necessary for byte
// alignment to process subsequent messages.
// https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md#the-first-object-the-trace-object
func parseTraceMessage(r network.MultiReader) (err error) {
	type eventSource struct {
		syncTimeQPC             int64
		qpcFreq                 int64
		pointerSize             int32
		processID               int32
		numProcessors           int32
		expectedCPUSamplingRate int32
	}

	es := eventSource{}

	// skip date
	const dateSize = 16
	err = r.Seek(dateSize)
	if err != nil {
		return
	}

	err = r.Read(&es.syncTimeQPC)
	if err != nil {
		return
	}

	err = r.Read(&es.qpcFreq)
	if err != nil {
		return
	}

	err = r.Read(&es.pointerSize)
	if err != nil {
		return
	}

	err = r.Read(&es.processID)
	if err != nil {
		return
	}

	err = r.Read(&es.numProcessors)
	if err != nil {
		return
	}

	err = r.Read(&es.expectedCPUSamplingRate)

	return
}
