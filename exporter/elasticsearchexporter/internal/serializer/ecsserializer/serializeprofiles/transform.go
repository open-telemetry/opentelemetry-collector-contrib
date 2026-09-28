// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/ecsserializer/serializeprofiles"

import (
	"fmt"
	"math"
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
		frames, frameTypes, err := serializer.StackFrames(dic, sample)
		if err != nil {
			return nil, fmt.Errorf("failed to create stackframes: %w", err)
		}
		if len(frames) == 0 {
			continue
		}

		traceID := serializer.StackTraceID(frames)
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

		if leaf := &frames[len(frames)-1]; !leaf.IsSymbolized() {
			unsymbolizedLeafFramesSet[leaf.ID] = struct{}{}
		}

		for j := range frames {
			if frameTypes[j].IsError() {
				// Artificial error frames can't be symbolized.
				continue
			}
			if frames[j].IsSymbolized() {
				// Skip interpreted frames and already symbolized native frames (kernel, Golang is planned).
				continue
			}
			unsymbolizedExecutablesSet[frames[j].ID.FileID()] = struct{}{}
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
func symbolizedFrames(frames []serializer.Frame) []StackFrame {
	framesWithSymbols := make([]StackFrame, 0, len(frames))
	for i := range frames {
		f := &frames[i]
		if !f.IsSymbolized() {
			continue
		}
		framesWithSymbols = append(framesWithSymbols, StackFrame{
			EcsVersion:   EcsVersion{V: EcsVersionString},
			DocID:        f.DocID,
			FileName:     f.FileName,
			FunctionName: f.FunctionName,
			LineNumber:   f.LineNumber,
		})
	}
	return framesWithSymbols
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
		if int(idx) < 0 || int(idx) >= dic.AttributeTable().Len() {
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

func stackTrace(stackTraceID string, frames []serializer.Frame, frameTypes []libpf.FrameType) StackTrace {
	frameIDs, types := serializer.EncodeStackTrace(frames, frameTypes)

	return StackTrace{
		EcsVersion: EcsVersion{V: EcsVersionString},
		DocID:      stackTraceID,
		FrameIDs:   frameIDs,
		Types:      types,
	}
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
