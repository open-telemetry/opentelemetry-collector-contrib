// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

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

		ts := serializer.NewUnixTime64(uint64(time.Now().UnixNano()))
		if sample.TimestampsUnixNano().Len() > 0 {
			ts = serializer.NewUnixTime64(sample.TimestampsUnixNano().At(0))
		}

		// Set the stacktrace and stackframes to the payload.
		// The docs only need to be written once.
		stackPayload = append(stackPayload, StackPayload{
			StackTrace:  stackTrace(traceID, frames, frameTypes, ts),
			StackFrames: symbolizedFrames(frames, ts),
			ResourceAttrs: ResourceData{
				Data: commonResourceAttributes,
			},
		})

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
			stackPayload[0].Executables = executables(dic, dic.MappingTable())
		}
	}

	return stackPayload, nil
}

// symbolizedFrames returns a slice of StackFrames that have symbols.
func symbolizedFrames(frames []serializer.Frame, ts serializer.UnixTime64) []StackFrame {
	framesWithSymbols := make([]StackFrame, 0, len(frames))
	for i := range frames {
		f := &frames[i]
		if !f.IsSymbolized() {
			continue
		}
		framesWithSymbols = append(framesWithSymbols, StackFrame{
			DocID:        f.DocID,
			Timestamp:    ts,
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
		HostID:           commonResourceAttrs[string(conventions.HostIDKey)],
		StackTraceID:     traceID,
		ContainerID:      commonResourceAttrs[string(conventions.ContainerIDKey)],
		ContainerName:    commonResourceAttrs[string(conventions.ContainerNameKey)],
		PodName:          commonResourceAttrs[string(conventions.K8SPodNameKey)],
		K8sNamespaceName: commonResourceAttrs[string(conventions.K8SNamespaceNameKey)],
		Count:            1,
		Frequency:        frequency,
		HostName:         commonResourceAttrs[string(conventions.HostNameKey)],
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

func stackTrace(stackTraceID string, frames []serializer.Frame, frameTypes []libpf.FrameType, ts serializer.UnixTime64) StackTrace {
	frameIDs, types := serializer.EncodeStackTrace(frames, frameTypes)

	return StackTrace{
		DocID:     stackTraceID,
		Timestamp: ts,
		FrameIDs:  frameIDs,
		Types:     types,
	}
}

func executables(dic pprofile.ProfilesDictionary, mappings pprofile.MappingSlice) []ExeMetadata {
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

		buildIDStr, err := serializer.GetStringFromAttribute(dic, mapping, string(conventions.ProcessExecutableBuildIDHtlhashKey))
		if err != nil || buildIDStr == "" {
			// No build ID was specified or could be fetched.
			continue
		}

		metadata = append(metadata, ExeMetadata{
			DocID:     buildIDStr,
			Timestamp: lastSeen,
			BuildID:   buildIDStr,
			Name:      filename,
		})
	}

	return metadata
}
