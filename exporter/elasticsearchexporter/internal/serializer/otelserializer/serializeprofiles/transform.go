// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
)

// Transform transforms a [pprofile.Profile] into our own
// representation, for ingestion into Elasticsearch
func Transform(resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile) ([]StackPayload, error) {
	var data []StackPayload

	if err := checkProfileType(profile); err != nil {
		return data, err
	}

	// profileContainer is checked for nil inside stackPayloads().
	payloads, err := stackPayloads(resource, scope, profile)
	if err != nil {
		return nil, err
	}
	data = append(data, payloads...)

	return data, nil
}

// checkProfileType acts as safeguard to make sure only known profiles are
// accepted. Different kinds of profiles are currently not supported
// and mixing profiles will make profiling information unusable.
func checkProfileType(profile pprofile.Profile) error {
	sampleType := profile.SampleType()
	if sampleType.Len() != 1 {
		return fmt.Errorf("expected 1 sample type but got %d", sampleType.Len())
	}

	sType := getString(profile, int(sampleType.At(0).TypeStrindex()))
	sUnit := getString(profile, int(sampleType.At(0).UnitStrindex()))

	// Make sure only on-CPU profiling data is accepted at the moment.
	// This needs to match with
	//nolint:lll
	// https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/a720d06a401cb23249c5066dc69e96384af99cf3/reporter/otlp_reporter.go#L531
	if !strings.EqualFold(sType, "samples") || !strings.EqualFold(sUnit, "count") {
		return fmt.Errorf("expected sampling type of  [[\"samples\",\"count\"]] "+
			"but got [[\"%s\", \"%s\"]]", sType, sUnit)
	}

	periodType := profile.PeriodType()
	pType := getString(profile, int(periodType.TypeStrindex()))
	pUnit := getString(profile, int(periodType.UnitStrindex()))

	// Make sure only on-CPU profiling data is accepted at the moment.
	// This needs to match with
	//nolint:lll
	// https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/a720d06a401cb23249c5066dc69e96384af99cf3/reporter/otlp_reporter.go#L536
	if !strings.EqualFold(pType, "cpu") || !strings.EqualFold(pUnit, "nanoseconds") {
		return fmt.Errorf("expected period type [\"cpu\",\"nanoseconds\"] but got "+
			"[\"%s\", \"%s\"]", pType, pUnit)
	}

	return nil
}

// stackPayloads creates a slice of StackPayloads from the given ResourceProfiles,
// ScopeProfiles, and ProfileContainer.
func stackPayloads(resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile) ([]StackPayload, error) {
	unsymbolizedLeafFrames := make([]libpf.FrameID, 0, profile.Sample().Len())
	stackPayload := make([]StackPayload, 0, profile.Sample().Len())

	hostMetadata := newHostMetadata(resource, scope, profile)

	for i := 0; i < profile.Sample().Len(); i++ {
		sample := profile.Sample().At(i)

		frames, frameTypes, leafFrame, err := stackFrames(profile, sample)
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

		event := stackTraceEvent(traceID, profile, sample, hostMetadata)

		// Set the stacktrace and stackframes to the payload.
		// The docs only need to be written once.
		stackPayload = append(stackPayload, StackPayload{
			StackTrace:  stackTrace(traceID, frames, frameTypes),
			StackFrames: symbolizedFrames(frames),
		})

		if !isFrameSymbolized(frames[len(frames)-1]) && leafFrame != nil {
			unsymbolizedLeafFrames = append(unsymbolizedLeafFrames, *leafFrame)
		}

		// Add one event per timestamp and its count value.
		for j := 0; j < sample.TimestampsUnixNano().Len(); j++ {
			t := sample.TimestampsUnixNano().At(j)
			event.TimeStamp = newUnixTime64(t)

			if j < sample.Value().Len() {
				event.Count = uint16(sample.Value().At(j))
			} else {
				event.Count = 1 // restore default
			}
			if event.Count > 0 {
				stackPayload = append(stackPayload, StackPayload{
					StackTraceEvent: event,
				})
			}
		}
	}

	if len(stackPayload) > 0 {
		if profile.MappingTable().Len() > 0 {
			exeMetadata, err := executables(profile, profile.MappingTable())
			if err != nil {
				return nil, err
			}

			stackPayload[0].Executables = exeMetadata
		}
		stackPayload[0].UnsymbolizedLeafFrames = unsymbolizedLeafFrames
	}

	return stackPayload, nil
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

func stackTraceEvent(traceID string, profile pprofile.Profile, sample pprofile.Sample, hostMetadata map[string]string) StackTraceEvent {
	event := StackTraceEvent{
		EcsVersion:   EcsVersion{V: EcsVersionString},
		HostID:       hostMetadata[string(semconv.HostIDKey)],
		StackTraceID: traceID,
		Count:        1, // TODO: Check whether count can be dropped with nanosecond timestamps
	}

	// Store event-specific attributes.
	for i := 0; i < sample.AttributeIndices().Len(); i++ {
		if profile.AttributeTable().Len() < i {
			continue
		}
		attr := profile.AttributeTable().At(i)

		switch attribute.Key(attr.Key()) {
		case semconv.HostIDKey:
			event.HostID = attr.Value().AsString()
		case semconv.ContainerIDKey:
			event.ContainerID = attr.Value().AsString()
		case semconv.K8SPodNameKey:
			event.PodName = attr.Value().AsString()
		case semconv.ContainerNameKey:
			event.ContainerName = attr.Value().AsString()
		case semconv.ThreadNameKey:
			event.ThreadName = attr.Value().AsString()
		}
	}

	return event
}

func stackTrace(stackTraceID string, frames []StackFrame, frameTypes []libpf.FrameType) StackTrace {
	frameIDs := make([]string, 0, len(frames))
	for _, f := range frames {
		frameIDs = append(frameIDs, f.DocID)
	}

	// Up to 255 consecutive identical frame types are converted into 2 bytes (binary).
	// We expect mostly consecutive frame types in a trace. Even if the encoding
	// takes more than 32 bytes in single cases, the probability that the average base64 length
	// per trace is below 32 bytes is very high.
	// We expect resizing of buf to happen very rarely.
	buf := bytes.NewBuffer(make([]byte, 0, 32))
	encodeFrameTypesTo(buf, frameTypes)

	return StackTrace{
		EcsVersion: EcsVersion{V: EcsVersionString},
		DocID:      stackTraceID,
		FrameIDs:   strings.Join(frameIDs, ""),
		Types:      buf.String(),
	}
}

func stackFrames(profile pprofile.Profile, sample pprofile.Sample) ([]StackFrame, []libpf.FrameType, *libpf.FrameID, error) {
	frames := make([]StackFrame, 0, sample.LocationsLength())

	locations := getLocations(profile, sample)
	totalFrames := 0
	for _, location := range locations {
		totalFrames += location.Line().Len()
	}
	frameTypes := make([]libpf.FrameType, 0, totalFrames)

	var leafFrameID *libpf.FrameID

	for locationIdx, location := range locations {
		if location.MappingIndex() >= int32(profile.MappingTable().Len()) {
			continue
		}

		frameTypeStr, err := getStringFromAttribute(profile, location, "profile.frame.type")
		if err != nil {
			return nil, nil, nil, err
		}
		frameTypes = append(frameTypes, libpf.FrameTypeFromString(frameTypeStr))

		functionNames := make([]string, 0, location.Line().Len())
		fileNames := make([]string, 0, location.Line().Len())
		lineNumbers := make([]int32, 0, location.Line().Len())

		for i := 0; i < location.Line().Len(); i++ {
			line := location.Line().At(i)

			if line.FunctionIndex() < int32(profile.FunctionTable().Len()) {
				functionNames = append(functionNames, getString(profile, int(profile.FunctionTable().At(int(line.FunctionIndex())).NameStrindex())))
				fileNames = append(fileNames, getString(profile, int(profile.FunctionTable().At(int(line.FunctionIndex())).FilenameStrindex())))
			}
			lineNumbers = append(lineNumbers, int32(line.Line()))
		}

		frameID, err := getFrameID(profile, location)
		if err != nil {
			return nil, nil, nil, err
		}

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

func getFrameID(profile pprofile.Profile, location pprofile.Location) (*libpf.FrameID, error) {
	// The MappingIndex is known to be valid.
	mapping := profile.MappingTable().At(int(location.MappingIndex()))
	buildID, err := getBuildID(profile, mapping)
	if err != nil {
		return nil, err
	}

	var addressOrLineno uint64
	if location.Address() > 0 {
		addressOrLineno = location.Address()
	} else if location.Line().Len() > 0 {
		addressOrLineno = uint64(location.Line().At(location.Line().Len() - 1).Line())
	}

	frameID := libpf.NewFrameID(buildID, libpf.AddressOrLineno(addressOrLineno))
	return &frameID, nil
}

type attributable interface {
	AttributeIndices() pcommon.Int32Slice
}

// getStringFromAttribute returns a string from one of attrIndices from the attribute table
// of the profile if the attribute key matches the expected attrKey.
func getStringFromAttribute(profile pprofile.Profile, record attributable, attrKey string) (string, error) {
	lenAttrTable := profile.AttributeTable().Len()

	for i := 0; i < record.AttributeIndices().Len(); i++ {
		idx := int(record.AttributeIndices().At(i))

		if idx >= lenAttrTable {
			return "", fmt.Errorf("requested attribute index (%d) "+
				"exceeds size of attribute table (%d)", idx, lenAttrTable)
		}
		if profile.AttributeTable().At(idx).Key() == attrKey {
			return profile.AttributeTable().At(idx).Value().AsString(), nil
		}
	}

	return "", fmt.Errorf("failed to get '%s' from indices %v", attrKey, record.AttributeIndices().AsRaw())
}

// getBuildID returns the Build ID for the given mapping. It checks for both
// old-style Build ID (stored with the mapping) and Build ID as attribute.
func getBuildID(profile pprofile.Profile, mapping pprofile.Mapping) (libpf.FileID, error) {
	// Fetch build ID from profiles.attribute_table.
	buildIDStr, err := getStringFromAttribute(profile, mapping, "process.executable.build_id.htlhash")
	if err != nil {
		return libpf.FileID{}, err
	}
	return libpf.FileIDFromString(buildIDStr)
}

func executables(profile pprofile.Profile, mappings pprofile.MappingSlice) ([]ExeMetadata, error) {
	metadata := make([]ExeMetadata, 0, mappings.Len())
	lastSeen := GetStartOfWeekFromTime(time.Now())

	for i := 0; i < mappings.Len(); i++ {
		mapping := mappings.At(i)

		filename := profile.StringTable().At(int(mapping.FilenameStrindex()))
		if filename == "" {
			// This is true for interpreted languages like Python.
			continue
		}

		buildID, err := getBuildID(profile, mapping)
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
	for i := len(frames) - 1; i >= 0; i-- { // reverse ordered frames, done in stackFrames()
		frameID, err := libpf.NewFrameIDFromString(frames[i].DocID)
		if err != nil {
			return "", fmt.Errorf("failed to create frameID from string: %w", err)
		}
		_, _ = h.Write(frameID.FileID().Bytes())
		// Using FormatUint() or putting AppendUint() into a function leads
		// to escaping to heap (allocation).
		_, _ = h.Write(strconv.AppendUint(buf[:0], uint64(frameID.AddressOrLine()), 10))
	}
	// make instead of nil avoids a heap allocation
	traceHash, err := libpf.TraceHashFromBytes(h.Sum(make([]byte, 0, 16)))
	if err != nil {
		return "", err
	}

	return traceHash.Base64(), nil
}

func getLocations(profile pprofile.Profile, sample pprofile.Sample) []pprofile.Location {
	if sample.LocationsLength() > 0 {
		locations := make([]pprofile.Location, 0, sample.LocationsLength())

		for i := int(sample.LocationsStartIndex()); i < int(sample.LocationsLength()); i++ {
			if i < profile.LocationTable().Len() {
				locations = append(locations, profile.LocationTable().At(i))
			}
		}
		return locations
	}

	locations := make([]pprofile.Location, 0, sample.LocationsLength())
	lastIndex := int(sample.LocationsStartIndex() + sample.LocationsLength())
	for i := int(sample.LocationsStartIndex()); i < lastIndex; i++ {
		if i < profile.LocationTable().Len() {
			locations = append(locations, profile.LocationTable().At(i))
		}
	}
	return locations
}

func getString(profile pprofile.Profile, index int) string {
	if index < profile.StringTable().Len() {
		return profile.StringTable().At(index)
	}
	return ""
}

func GetStartOfWeekFromTime(t time.Time) uint32 {
	return uint32(t.Truncate(time.Hour * 24 * 7).Unix())
}

func newHostMetadata(resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile) map[string]string {
	attrs := make(map[string]string, 128)

	addEventHostData(attrs, resource.Attributes())
	addEventHostData(attrs, scope.Attributes())
	addEventHostData(attrs, pprofile.FromAttributeIndices(profile.AttributeTable(), profile))

	if len(attrs) == 0 {
		return nil
	}

	return attrs
}

func addEventHostData(data map[string]string, attrs pcommon.Map) {
	attrs.Range(func(k string, v pcommon.Value) bool {
		data[k] = v.AsString()
		return true
	})
}
