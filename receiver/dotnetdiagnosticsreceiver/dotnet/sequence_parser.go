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

import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"

// parseSPBlock parses a Sequence Point object. The contents are not used, but
// reading the correct number of bytes is necessary for processing subsequent
// messages. When this message is encountered, the caller may, for example,
// reset its byte counter to prevent overflow.
// https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md#sequencepointblock-object
func parseSPBlock(r network.MultiReader) error {
	var offset int32
	err := r.Read(&offset)
	if err != nil {
		return err
	}

	err = r.Align()
	if err != nil {
		return err
	}

	var timestamp int64
	err = r.Read(&timestamp)
	if err != nil {
		return err
	}

	var threadcount int32
	err = r.Read(&threadcount)
	if err != nil {
		return err
	}

	for i := 0; i < int(threadcount); i++ {
		var captureThreadID int64
		err = r.Read(&captureThreadID)
		if err != nil {
			return err
		}

		var sequenceNumber int32
		err = r.Read(&sequenceNumber)
		if err != nil {
			return err
		}
	}
	return nil
}
