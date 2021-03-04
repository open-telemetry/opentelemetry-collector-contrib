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

// fieldMetadataMap is a map of fieldMetadata by metadataID
type fieldMetadataMap map[int]fieldMetadata

// fieldMetadata holds a tree of field metadata used to parse event data.
// See fms() in event_parser_test.go for an example
type fieldMetadata struct {
	header metadataHeader
	fields []field
}

// only metadataID is used
type metadataHeader struct {
	metadataID    int32
	providerName  string
	eventHeaderID int32
	eventName     string
	keyword       uint64
	version       int32
	level         int32
}

// field contains metadata about a key-value pair extracted from an event message,
// used to populate a Metric
type field struct {
	name      string
	fieldType fieldType
	fields    []field
}

type fieldType string

const (
	fieldTypeStruct fieldType = "Struct"
	fieldTypeString fieldType = "String"
	fieldTypeInt32  fieldType = "Int32"
	fieldTypeSingle fieldType = "Single"
	fieldTypeDouble fieldType = "Double"
)

// parseMetadataBlock parses a metadata block and populates the passed-in
// fieldMetadataMap struct.
// https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md#the-metadatablock-object
func parseMetadataBlock(r network.MultiReader, fmm fieldMetadataMap) error {
	var offset int32
	err := r.Read(&offset)
	if err != nil {
		return err
	}

	endpos := r.Pos() + int(offset)

	err = r.Align()
	if err != nil {
		return err
	}

	var headerSize int16
	err = r.Read(&headerSize)
	if err != nil {
		return err
	}

	var flags int16
	err = r.Read(&flags)
	if err != nil {
		return err
	}

	err = r.Seek(16)
	if err != nil {
		return err
	}

	for r.Pos() < endpos {
		fm, err := parseFieldMetadata(r)
		if err != nil {
			return err
		}

		fmm[int(fm.header.metadataID)] = fm
	}
	return nil
}

func parseFieldMetadata(r network.MultiReader) (m fieldMetadata, err error) {
	ignored := eventHeader{}
	err = parseEventHeader(r, &ignored)
	if err != nil {
		return
	}

	m.header, err = parseMetadataHeader(r)
	if err != nil {
		return
	}

	m.fields, err = parseFields(r)
	if err != nil {
		return
	}

	return
}

func parseMetadataHeader(r network.MultiReader) (h metadataHeader, err error) {
	err = r.Read(&h.metadataID)
	if err != nil {
		return
	}

	h.providerName, err = r.ReadUTF16()
	if err != nil {
		return
	}

	err = r.Read(&h.eventHeaderID)
	if err != nil {
		return
	}

	h.eventName, err = r.ReadUTF16()
	if err != nil {
		return
	}

	err = r.Read(&h.keyword)
	if err != nil {
		return
	}

	err = r.Read(&h.version)
	if err != nil {
		return
	}

	err = r.Read(&h.level)
	return
}

func parseFields(r network.MultiReader) (fields []field, err error) {
	var numFields int32
	err = r.Read(&numFields)
	if err != nil {
		return
	}

	for i := 0; i < int(numFields); i++ {
		f, err := parseField(r)
		if err != nil {
			return nil, err
		}
		f.name, err = r.ReadUTF16()
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return
}

// from https://docs.microsoft.com/en-us/dotnet/api/system.typecode
const (
	typeCodeObject = 1
	typeCodeInt32  = 9
	typeCodeSingle = 13
	typeCodeDouble = 14
	typeCodeString = 18
)

func parseField(r network.MultiReader) (f field, err error) {
	// EventPipeEventSource#ParseType
	var typeCode int32
	err = r.Read(&typeCode)
	if err != nil {
		return
	}

	switch typeCode {
	case typeCodeObject:
		f.fields, err = parseFields(r)
		if err != nil {
			return
		}
		f.fieldType = fieldTypeStruct
	case typeCodeInt32:
		f.fieldType = fieldTypeInt32
	case typeCodeSingle:
		f.fieldType = fieldTypeSingle
	case typeCodeDouble:
		f.fieldType = fieldTypeDouble
	case typeCodeString:
		f.fieldType = fieldTypeString
	}
	return
}
