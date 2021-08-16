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

type eventHeader struct {
	metadataID        uint32
	sequenceNumber    int32
	captureThreadID   int64
	captureProcNumber int32
	threadID          int64
	stackID           int32
	payloadSize       int32
	timestampDelta    int64
}

type headerFlags byte

// from CompressedHeaderFlags enum in EventPipeEventSource.cs
// https://github.com/microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeEventSource.cs#L1382
const (
	headerFlagMetadataID               headerFlags = 1 << 0
	headerFlagCaptureThreadAndSequence headerFlags = 1 << 1
	headerFlagThreadID                 headerFlags = 1 << 2
	headerFlagStackID                  headerFlags = 1 << 3
	headerFlagActivityID               headerFlags = 1 << 4
	headerFlagRelatedActivityID        headerFlags = 1 << 5
	headerFlagDataLength               headerFlags = 1 << 7
)

func (f headerFlags) isSet(other headerFlags) bool {
	return f&other != 0
}

// parseEventHeader is used by event parser (and by metadata parser for stream
// alignment only) to get the metadata ID so that it can be correlated to the
// extracted metadata. The rest of the extracted information is currently
// unused.
func parseEventHeader(r network.MultiReader, h *eventHeader) (err error) {
	// EventPipeEventHeader.ReadFromFormatV4
	var b byte
	b, err = r.ReadByte()
	if err != nil {
		return
	}
	f := headerFlags(b)

	if f.isSet(headerFlagMetadataID) {
		h.metadataID, err = r.ReadCompressedUInt32()
		if err != nil {
			return
		}
	}

	if f.isSet(headerFlagCaptureThreadAndSequence) {
		h.sequenceNumber, err = r.ReadCompressedInt32()
		if err != nil {
			return
		}

		h.captureThreadID, err = r.ReadCompressedInt64()
		if err != nil {
			return
		}

		h.captureProcNumber, err = r.ReadCompressedInt32()
		if err != nil {
			return
		}
	}

	h.sequenceNumber++

	if f.isSet(headerFlagThreadID) {
		h.threadID, err = r.ReadCompressedInt64()
		if err != nil {
			return
		}
	}

	if f.isSet(headerFlagStackID) {
		h.stackID, err = r.ReadCompressedInt32()
		if err != nil {
			return
		}
	}

	h.timestampDelta, err = r.ReadCompressedInt64()
	if err != nil {
		return
	}

	const guidSize = 16
	if f.isSet(headerFlagActivityID) {
		// skip guid
		err = r.Seek(guidSize)
		if err != nil {
			return
		}
	}

	if f.isSet(headerFlagRelatedActivityID) {
		// skip guid
		err = r.Seek(guidSize)
		if err != nil {
			return
		}
	}

	if f.isSet(headerFlagDataLength) {
		h.payloadSize, err = r.ReadCompressedInt32()
		if err != nil {
			return
		}
	}

	return
}
