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

// parseStackBlock parses a stack block. The results are currently not used, but
// skippping over the correct number of bytes is required for parsing subsequent
// messages.
// https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md#stackblock-object
func parseStackBlock(r network.MultiReader) error {
	var offset int32
	err := r.Read(&offset)
	if err != nil {
		return err
	}

	curr := r.Pos()
	endpos := curr + int(offset)

	err = r.Align()
	if err != nil {
		return err
	}

	var firstStackID int32
	err = r.Read(&firstStackID)
	if err != nil {
		return err
	}

	var countStackIDs int32
	err = r.Read(&countStackIDs)
	if err != nil {
		return err
	}

	for r.Pos() < endpos {
		var stackBytesSize int32
		err = r.Read(&stackBytesSize)
		if err != nil {
			return err
		}

		err = r.Seek(int(stackBytesSize))
		if err != nil {
			return err
		}
	}
	return nil
}
