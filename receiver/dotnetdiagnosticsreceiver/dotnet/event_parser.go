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

// parseEventBlock parses an event block and returns a Metric slice containing
// the raw representation of the metrics extracted from the event messages. It
// uses the structure and names of the passed-in fieldMetadataMap (from
// parseMetadataBlock) and the values extracted from the stream to build the
// Metrics and their key-value pairs.
// https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md#the-eventblock-object
func parseEventBlock(r network.MultiReader, fm fieldMetadataMap) (metrics []Metric, err error) {
	var offset int32
	err = r.Read(&offset)
	if err != nil {
		return
	}

	err = r.Align()
	if err != nil {
		return
	}

	endpos := r.Pos() + int(offset)

	// EventCache#ProcessEventBlock
	var headerSize uint16
	err = r.Read(&headerSize)
	if err != nil {
		return
	}

	var flags uint16
	err = r.Read(&flags)
	if err != nil {
		return
	}

	// subtract the four bytes we just read
	err = r.Seek(int(headerSize) - 4)
	if err != nil {
		return
	}

	header := eventHeader{}
	for r.Pos() < endpos {
		err = parseEventHeader(r, &header)
		if err != nil {
			return
		}

		m := Metric{}
		// here we correlate the metadata extracted from parseMetadataBlock to the events
		// contained in this message
		err = parseFieldValues(fm[int(header.metadataID)].fields, r, m)
		if err != nil {
			return
		}
		if len(m) > 0 {
			metrics = append(metrics, m)
		}
	}

	return
}

// parseFieldValues recursively populates a Metric using metadata fields and a
// MultiReader.
func parseFieldValues(fields []field, r network.MultiReader, m Metric) error {
	for _, field := range fields {
		// These are all of the types encountered during testing thus far.
		// TODO look for use cases that require additional types
		switch field.fieldType {
		case fieldTypeStruct:
			// recursive call to child fields if this is a struct
			err := parseFieldValues(field.fields, r, m)
			if err != nil {
				return err
			}
		case fieldTypeString:
			v, err := r.ReadUTF16()
			if err != nil {
				return err
			}
			m[field.name] = v
		case fieldTypeDouble:
			var v float64
			err := r.Read(&v)
			if err != nil {
				return err
			}
			m[field.name] = v
		case fieldTypeSingle:
			var v float32
			err := r.Read(&v)
			if err != nil {
				return err
			}
			m[field.name] = v
		case fieldTypeInt32:
			var v int32
			err := r.Read(&v)
			if err != nil {
				return err
			}
			m[field.name] = v
		}
	}
	return nil
}
