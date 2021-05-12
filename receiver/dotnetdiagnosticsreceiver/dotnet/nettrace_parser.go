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
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

const (
	nettraceName          = "Nettrace"
	nettraceSerialization = "!FastSerialization.1"
)

type nettrace struct {
	name              string
	serializationType string
}

// Parses and validates a nettrace message:
// https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md#first-bytes-nettrace-magic
func parseNettrace(r network.MultiReader) error {
	nt, err := doParseNettrace(r)
	if err != nil {
		return err
	}
	return validateNettrace(nt)
}

func doParseNettrace(r network.MultiReader) (h nettrace, err error) {
	h.name, err = r.ReadASCII(len(nettraceName))
	if err != nil {
		return
	}
	var strlen int32
	err = r.Read(&strlen)
	if err != nil {
		return
	}
	h.serializationType, err = r.ReadASCII(int(strlen))
	return
}

func validateNettrace(h nettrace) error {
	if h.name != nettraceName {
		return fmt.Errorf(
			"header name: expected %q got %q",
			nettraceName,
			h.name,
		)
	}
	if h.serializationType != nettraceSerialization {
		return fmt.Errorf(
			"serialization type: expected %q got %q",
			nettraceSerialization,
			h.serializationType,
		)
	}
	return nil
}
