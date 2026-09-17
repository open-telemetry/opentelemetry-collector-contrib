// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/ecsserializer/serializeprofiles"

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/otel/attribute"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"
)

// Transform transforms a [pprofile.Profile] into our own
// representation, for ingestion into Elasticsearch
func Transform(dic pprofile.ProfilesDictionary, resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile) ([]StackPayload, error) {
	var data []StackPayload

	if err := serializer.CheckProfileType(dic, profile); err != nil {
		return data, err
	}

	// profileContainer is checked for nil inside stackPayloads().
	payloads, err := stackPayloads(dic, resource, scope, profile)
	if err != nil {
		return nil, err
	}
	data = append(data, payloads...)

	return data, nil
}

// stackPayloads creates a slice of StackPayloads from the given ResourceProfiles,
// ScopeProfiles, and ProfileContainer.
func stackPayloads(dic pprofile.ProfilesDictionary, resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile) ([]StackPayload, error) {
	unsymbolizedLeafFramesSet := make(map[serializer.FrameID]struct{}, profile.Samples().Len())
	unsymbolizedExecutablesSet := make(map[libpf.FileID]struct{})
	stackPayload := make([]StackPayload, 0, profile.Samples().Len())

	commonResourceAttributes, err := serializer.PopulateResourceData(dic, resource, scope, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to populate resource data: %w", err)
	}

	frequency := int64(math.Round(1e9 / float64(profile.Period())))
	if frequency <= 0 {
		// The lowest sensical frequency is 1Hz.
		frequency = 1
	}

	for _, sample := range profile.Samples().All() {
		frames, frameTypes, leafFrame, err := stackFrames(dic, sample)
		if err != nil {
			return nil, fmt.Errorf("failed to create stackframes: %w", err)
		}
		if len(frames) == 0 {
			continue
		}

		traceID, err := stackTraceID(frames)
		if err != nil {
			return nil, fmt.Errorf("failed to create stacktrace ID: %w", err)
		}

		event := stackTraceEvent(dic, traceID, sample, frequency, commonResourceAttributes)

		// Set the stacktrace and stackframes to the payload.
		// The docs only need to be written once.
		stackPayload = append(stackPayload, StackPayload{
			StackTrace:  stackTrace(traceID, frames, frameTypes),
			StackFrames: symbolizedFrames(frames),
			ResourceAttrs: ResourceData{
				EcsVersion: EcsVersion{
					V: EcsVersionString,
				},
				Data: commonResourceAttributes,
			},
		})

		if !isFrameSymbolized(frames[len(frames)-1]) && leafFrame != nil {
			unsymbolizedLeafFramesSet[*leafFrame] = struct{}{}
		}

		for j := range frames {
			if frameTypes[j].IsError() {
				// Artificial error frames can't be symbolized.
				continue
			}
			if isFrameSymbolized(frames[j]) {
				// Skip interpreted frames and already symbolized native frames (kernel, Golang is planned).
				continue
			}
			fID, err := serializer.NewFrameIDFromString(frames[j].DocID)
			if err != nil {
				return nil, fmt.Errorf("stackPayloads: %w", err)
			}
			unsymbolizedExecutablesSet[fID.FileID()] = struct{}{}
		}

		// Add one event per timestamp and its count value.
		for j, t := range sample.TimestampsUnixNano().All() {
			event.TimeStamp = serializer.NewUnixTime64(t)

			count := 1
			if j < sample.Values().Len() {
				count = int(sample.Values().At(j))
			}
			for range count {
				stackPayload = append(stackPayload, StackPayload{
					StackTraceEvent: event,
				})
			}
		}
	}

	if len(stackPayload) > 0 {
		if dic.MappingTable().Len() > 0 {
			exeMetadata, err := executables(dic, dic.MappingTable())
			if err != nil {
				return nil, err
			}

			stackPayload[0].Executables = exeMetadata
		}
		stackPayload[0].UnsymbolizedLeafFrames = unsymbolizedLeafFrames(unsymbolizedLeafFramesSet)
		stackPayload[0].UnsymbolizedExecutables = unsymbolizedExecutables(unsymbolizedExecutablesSet)
	}

	return stackPayload, nil
}

func unsymbolizedExecutables(executables map[libpf.FileID]struct{}) []UnsymbolizedExecutable {
	now := time.Now()
	unsymbolized := make([]UnsymbolizedExecutable, 0, len(executables))
	for fileID := range executables {
		unsymbolized = append(unsymbolized, UnsymbolizedExecutable{
			EcsVersion: EcsVersion{V: EcsVersionString},
			DocID:      fileID.Base64(),
			FileID:     []string{fileID.Base64()},
			Created:    now,
			Next:       now,
			Retries:    0,
		})
	}
	return unsymbolized
}

func unsymbolizedLeafFrames(frameIDs map[serializer.FrameID]struct{}) []UnsymbolizedLeafFrame {
	now := time.Now()
	unsymbolized := make([]UnsymbolizedLeafFrame, 0, len(frameIDs))
	for frameID := range frameIDs {
		unsymbolized = append(unsymbolized, UnsymbolizedLeafFrame{
			EcsVersion: EcsVersion{V: EcsVersionString},
			DocID:      frameID.String(),
			FrameID:    []string{frameID.String()},
			Created:    now,
			Next:       now,
			Retries:    0,
		})
	}
	return unsymbolized
}

// symbolizedFrames returns a slice of StackFrames that have symbols.
func symbolizedFrames(frames []StackFrame) []StackFrame {
	framesWithSymbols := make([]StackFrame, 0, len(frames))
	for i := range frames {
		if isFrameSymbolized(frames[i]) {
			framesWithSymbols = append(framesWithSymbols, frames[i])
		}
	}
	return framesWithSymbols
}

func isFrameSymbolized(frame StackFrame) bool {
	return len(frame.FileName) > 0 || len(frame.FunctionName) > 0
}

func stackTraceEvent(dic pprofile.ProfilesDictionary, traceID string, sample pprofile.Sample, frequency int64,
	commonResourceAttrs map[string]string,
) StackTraceEvent {
	event := StackTraceEvent{
		EcsVersion:       EcsVersion{V: EcsVersionString},
		HostID:           commonResourceAttrs[string(conventions.HostIDKey)],
		StackTraceID:     traceID,
		ContainerID:      commonResourceAttrs[string(conventions.ContainerIDKey)],
		ContainerName:    commonResourceAttrs[string(conventions.ContainerNameKey)],
		PodName:          commonResourceAttrs[string(conventions.K8SPodNameKey)],
		K8sNamespaceName: commonResourceAttrs[string(conventions.K8SNamespaceNameKey)],
		Count:            1, // Elasticsearch v9.2+ doesn't read the count value any more.
		Frequency:        frequency,
		HostName:         commonResourceAttrs[string(conventions.HostNameKey)],
		ProjectID:        2, // Use a project ID other than 1 to not conflict with ECH default value.
		ServiceName:      commonResourceAttrs[string(conventions.ServiceNameKey)],
		ExecutableName:   commonResourceAttrs[string(conventions.ProcessExecutableNameKey)],
	}

	// Store event-specific attributes.
	for _, idx := range sample.AttributeIndices().All() {
		if dic.AttributeTable().Len() < int(idx) {
			continue
		}
		attr := dic.AttributeTable().At(int(idx))
		key := dic.StringTable().At(int(attr.KeyStrindex()))

		if attribute.Key(key) == conventions.ThreadNameKey {
			event.ThreadName = attr.Value().AsString()
		}
	}

	return event
}

func stackTrace(stackTraceID string, frames []StackFrame, frameTypes []libpf.FrameType) StackTrace {
	frameIDs := make([]string, 0, len(frames))
	for i := range frames {
		f := &frames[i]
		frameIDs = append(frameIDs, f.DocID)
	}

	// Up to 255 consecutive identical frame types are converted into 2 bytes (binary).
	// We expect mostly consecutive frame types in a trace. Even if the encoding
	// takes more than 32 bytes in single cases, the probability that the average base64 length
	// per trace is below 32 bytes is very high.
	// We expect resizing of buf to happen very rarely.
	buf := bytes.NewBuffer(make([]byte, 0, 32))
	serializer.EncodeFrameTypesTo(buf, frameTypes)

	return StackTrace{
		EcsVersion: EcsVersion{V: EcsVersionString},
		DocID:      stackTraceID,
		FrameIDs:   strings.Join(frameIDs, ""),
		Types:      buf.String(),
	}
}

func stackFrames(dic pprofile.ProfilesDictionary, sample pprofile.Sample) ([]StackFrame, []libpf.FrameType, *serializer.FrameID, error) {
	stack := dic.StackTable().At(int(sample.StackIndex()))
	frames := make([]StackFrame, 0, stack.LocationIndices().Len())

	locations := serializer.GetLocations(dic, stack)
	totalFrames := 0
	for _, location := range locations {
		totalFrames += location.Lines().Len()
	}
	frameTypes := make([]libpf.FrameType, 0, totalFrames)

	var leafFrameID *serializer.FrameID

	for locationIdx, location := range locations {
		if location.MappingIndex() >= int32(dic.MappingTable().Len()) {
			continue
		}

		frameTypeStr, err := serializer.GetStringFromAttribute(dic, location, string(conventions.ProfileFrameTypeKey))
		if err != nil {
			return nil, nil, nil, err
		}
		frameTypes = append(frameTypes, libpf.FrameTypeFromString(frameTypeStr))

		functionNames := make([]string, 0, location.Lines().Len())
		fileNames := make([]string, 0, location.Lines().Len())
		lineNumbers := make([]int32, 0, location.Lines().Len())

		for _, line := range location.Lines().All() {
			if line.FunctionIndex() < int32(dic.FunctionTable().Len()) {
				functionNames = append(functionNames, serializer.GetString(dic, int(dic.FunctionTable().At(int(line.FunctionIndex())).NameStrindex())))
				fileNames = append(fileNames, serializer.GetString(dic, int(dic.FunctionTable().At(int(line.FunctionIndex())).FilenameStrindex())))
			}
			lineNumbers = append(lineNumbers, int32(line.Line()))
		}

		frameID := serializer.GetFrameID(dic, location)

		if locationIdx == 0 {
			leafFrameID = frameID
		}

		frames = append([]StackFrame{
			{
				EcsVersion:   EcsVersion{V: EcsVersionString},
				DocID:        frameID.String(),
				FileName:     fileNames,
				FunctionName: functionNames,
				LineNumber:   lineNumbers,
			},
		}, frames...)
	}

	return frames, frameTypes, leafFrameID, nil
}

func executables(dic pprofile.ProfilesDictionary, mappings pprofile.MappingSlice) ([]ExeMetadata, error) {
	metadata := make([]ExeMetadata, 0, mappings.Len())
	lastSeen := serializer.GetStartOfWeekFromTime(time.Now())

	for i, mapping := range mappings.All() {
		if i == 0 {
			continue
		}

		filename := dic.StringTable().At(int(mapping.FilenameStrindex()))
		if filename == "" {
			// This is true for interpreted languages like Python.
			continue
		}

		buildID, err := serializer.GetBuildID(dic, mapping)
		if err != nil {
			return nil, err
		}

		if buildID.IsZero() {
			// No build ID was specified or could be fetched.
			continue
		}

		docID := buildID.Base64()
		executable := NewExeMetadata(docID, lastSeen, docID, filename)
		metadata = append(metadata, executable)
	}

	return metadata, nil
}

// stackTraceID creates a unique trace ID from the stack frames.
// For the OTEL profiling protocol, we have all required information in one wire message.
// But for the Elastic gRPC protocol, trace events and stack traces are sent separately, so
// that the host agent still needs to generate the stack trace IDs.
//
// The following code generates the same trace ID as the host agent.
// For ES 9.0.0, we could use a faster hash algorithm, e.g. xxh3, and hash strings instead
// of hashing binary data.
func stackTraceID(frames []StackFrame) (string, error) {
	var buf [24]byte
	h := fnv.New128a()
	for i := range slices.Backward(frames) { // reverse ordered frames, done in stackFrames()
		fID, err := serializer.NewFrameIDFromString(frames[i].DocID)
		if err != nil {
			return "", fmt.Errorf("failed to create frameID from string: %w", err)
		}
		_, _ = h.Write(fID.FileID().Bytes())
		// Using FormatUint() or putting AppendUint() into a function leads
		// to escaping to heap (allocation).
		_, _ = h.Write(strconv.AppendUint(buf[:0], uint64(fID.AddressOrLine()), 10))
	}
	// make instead of nil avoids a heap allocation
	traceHash, err := serializer.TraceHashFromBytes(h.Sum(make([]byte, 0, 16)))
	if err != nil {
		return "", err
	}

	return traceHash.Base64(), nil
}
