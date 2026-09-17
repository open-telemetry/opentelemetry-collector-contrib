// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"

import (
	"bytes"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// Frame is a stack frame before mode-specific serialization.
type Frame struct {
	ID FrameID
	// DocID is ID.String(), cached as it ends up in several documents.
	DocID        string
	FileName     []string
	FunctionName []string
	LineNumber   []int32
}

func (f *Frame) IsSymbolized() bool {
	return len(f.FileName) > 0 || len(f.FunctionName) > 0
}

// StackFrames returns the frames of a sample, root first. The frame types are
// returned leaf first, as EncodeFrameTypesTo expects them.
func StackFrames(dic pprofile.ProfilesDictionary, sample pprofile.Sample) ([]Frame, []libpf.FrameType, error) {
	stack := dic.StackTable().At(int(sample.StackIndex()))
	frames := make([]Frame, 0, stack.LocationIndices().Len())

	locations := GetLocations(dic, stack)
	totalFrames := 0
	for _, location := range locations {
		totalFrames += location.Lines().Len()
	}
	frameTypes := make([]libpf.FrameType, 0, totalFrames)

	for _, location := range locations {
		if location.MappingIndex() >= int32(dic.MappingTable().Len()) {
			continue
		}

		frameTypeStr, err := GetStringFromAttribute(dic, location, string(conventions.ProfileFrameTypeKey))
		if err != nil {
			return nil, nil, err
		}
		frameTypes = append(frameTypes, libpf.FrameTypeFromString(frameTypeStr))

		functionNames := make([]string, 0, location.Lines().Len())
		fileNames := make([]string, 0, location.Lines().Len())
		lineNumbers := make([]int32, 0, location.Lines().Len())

		for _, line := range location.Lines().All() {
			if line.FunctionIndex() < int32(dic.FunctionTable().Len()) {
				functionNames = append(functionNames, GetString(dic, int(dic.FunctionTable().At(int(line.FunctionIndex())).NameStrindex())))
				fileNames = append(fileNames, GetString(dic, int(dic.FunctionTable().At(int(line.FunctionIndex())).FilenameStrindex())))
			}
			lineNumbers = append(lineNumbers, int32(line.Line()))
		}

		frameID := GetFrameID(dic, location)

		frames = append([]Frame{{
			ID:           frameID,
			DocID:        frameID.String(),
			FileName:     fileNames,
			FunctionName: functionNames,
			LineNumber:   lineNumbers,
		}}, frames...)
	}

	return frames, frameTypes, nil
}

// StackTraceID creates a unique trace ID from the stack frames.
// For the OTEL profiling protocol, we have all required information in one wire message.
// But for the Elastic gRPC protocol, trace events and stack traces are sent separately, so
// that the host agent still needs to generate the stack trace IDs.
//
// The following code generates the same trace ID as the host agent.
// For ES 9.0.0, we could use a faster hash algorithm, e.g. xxh3, and hash strings instead
// of hashing binary data.
func StackTraceID(frames []Frame) string {
	var buf [24]byte
	h := fnv.New128a()
	for i := range slices.Backward(frames) { // reverse ordered frames, done in StackFrames()
		_, _ = h.Write(frames[i].ID.FileID().Bytes())
		// Using FormatUint() or putting AppendUint() into a function leads
		// to escaping to heap (allocation).
		_, _ = h.Write(strconv.AppendUint(buf[:0], uint64(frames[i].ID.AddressOrLine()), 10))
	}
	// make instead of nil avoids a heap allocation
	traceHash, _ := TraceHashFromBytes(h.Sum(make([]byte, 0, 16)))

	return traceHash.Base64()
}

// EncodeStackTrace returns the frame IDs and frame types in the format stored
// in the stacktraces index.
func EncodeStackTrace(frames []Frame, frameTypes []libpf.FrameType) (frameIDs, types string) {
	var ids strings.Builder
	ids.Grow(len(frames) * 32)
	for i := range frames {
		ids.WriteString(frames[i].DocID)
	}

	// Up to 255 consecutive identical frame types are converted into 2 bytes (binary).
	// We expect mostly consecutive frame types in a trace. Even if the encoding
	// takes more than 32 bytes in single cases, the probability that the average base64 length
	// per trace is below 32 bytes is very high.
	// We expect resizing of buf to happen very rarely.
	buf := bytes.NewBuffer(make([]byte, 0, 32))
	EncodeFrameTypesTo(buf, frameTypes)

	return ids.String(), buf.String()
}
