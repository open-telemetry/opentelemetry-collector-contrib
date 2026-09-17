// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// CheckProfileType acts as safeguard to make sure only known profiles are
// accepted. Different kinds of profiles are currently not supported
// and mixing profiles will make profiling information unusable.
func CheckProfileType(dic pprofile.ProfilesDictionary, profile pprofile.Profile) error {
	sampleType := profile.SampleType()

	sType := GetString(dic, int(sampleType.TypeStrindex()))
	sUnit := GetString(dic, int(sampleType.UnitStrindex()))

	// Make sure only on-CPU profiling data is accepted at the moment.
	// This needs to match with
	//nolint:lll
	// https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/a720d06a401cb23249c5066dc69e96384af99cf3/reporter/otlp_reporter.go#L531
	if !strings.EqualFold(sType, "samples") || !strings.EqualFold(sUnit, "count") {
		return fmt.Errorf("expected sampling type of  [[\"samples\",\"count\"]] "+
			"but got [[\"%s\", \"%s\"]]", sType, sUnit)
	}

	periodType := profile.PeriodType()
	pType := GetString(dic, int(periodType.TypeStrindex()))
	pUnit := GetString(dic, int(periodType.UnitStrindex()))

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

// GetFrameID derives the frame ID of a location from its mapping's build ID
// and address, synthesizing a file ID from the location's lines when no
// build ID is available.
func GetFrameID(dic pprofile.ProfilesDictionary, location pprofile.Location) *FrameID {
	// The MappingIndex is known to be valid.
	fileID := libpf.FileID{}

	if location.MappingIndex() > 0 {
		mapping := dic.MappingTable().At(int(location.MappingIndex()))
		fileID, _ = GetBuildID(dic, mapping)
	}
	if fileID.IsZero() {
		// Synthesize a file ID if the htlhash build ID is not available.
		hasher := xxhash.New()
		for _, line := range location.Lines().All() {
			f := GetFunction(dic, int(line.FunctionIndex()))
			_, _ = hasher.WriteString(GetString(dic, int(f.NameStrindex())))
			_, _ = hasher.WriteString(GetString(dic, int(f.FilenameStrindex())))
			_, _ = hasher.Write(int64ToBytes(line.Line()))
			_, _ = hasher.Write(int64ToBytes(line.Column()))
		}
		h := hasher.Sum64()
		fileID = libpf.NewFileID(h, h)
	}

	var addressOrLineno uint64
	if location.Address() > 0 {
		addressOrLineno = location.Address()
	} else if location.Lines().Len() > 0 {
		addressOrLineno = uint64(location.Lines().At(location.Lines().Len() - 1).Line())
	}

	fID := NewFrameID(fileID, libpf.AddressOrLineno(addressOrLineno))
	return &fID
}

type attributable interface {
	AttributeIndices() pcommon.Int32Slice
}

// ErrMissingAttribute allows to differentiate errors handling the AttributeTable
// and indicates that a attribute was not included in the AttributeTable.
var ErrMissingAttribute = errors.New("missing attribute")

// GetStringFromAttribute returns a string from one of attrIndices from the attribute table
// of the profile if the attribute key matches the expected attrKey.
func GetStringFromAttribute(dic pprofile.ProfilesDictionary, record attributable, attrKey string) (string, error) {
	lenAttrTable := dic.AttributeTable().Len()
	for _, idx32 := range record.AttributeIndices().All() {
		idx := int(idx32)

		if idx >= lenAttrTable {
			return "", fmt.Errorf("requested attribute index (%d) "+
				"exceeds size of attribute table (%d)", idx, lenAttrTable)
		}

		key := dic.StringTable().At(int(dic.AttributeTable().At(idx).KeyStrindex()))
		if key == attrKey {
			return dic.AttributeTable().At(idx).Value().AsString(), nil
		}
	}

	return "", fmt.Errorf("failed to get '%s': %w", attrKey, ErrMissingAttribute)
}

// GetBuildID returns the Build ID for the given mapping. It checks for both
// old-style Build ID (stored with the mapping) and Build ID as attribute.
// If the build ID attribute is missing, returns a zero FileID and no error.
func GetBuildID(dic pprofile.ProfilesDictionary, mapping pprofile.Mapping) (libpf.FileID, error) {
	// Fetch build ID from profiles.attribute_table.
	buildIDStr, err := GetStringFromAttribute(dic, mapping, string(conventions.ProcessExecutableBuildIDHtlhashKey))
	switch {
	case err == nil:
		return libpf.FileIDFromString(buildIDStr)
	case errors.Is(err, ErrMissingAttribute):
		return libpf.FileID{}, nil
	default:
		return libpf.FileID{}, err
	}
}

func GetLocations(dic pprofile.ProfilesDictionary, stack pprofile.Stack) []pprofile.Location {
	locations := make([]pprofile.Location, 0, stack.LocationIndices().Len())
	for _, i := range stack.LocationIndices().All() {
		locations = append(locations, dic.LocationTable().At(int(i)))
	}

	return locations
}

func GetString(dic pprofile.ProfilesDictionary, index int) string {
	if index < dic.StringTable().Len() {
		return dic.StringTable().At(index)
	}
	return ""
}

func GetFunction(dic pprofile.ProfilesDictionary, index int) pprofile.Function {
	if index < dic.FunctionTable().Len() {
		return dic.FunctionTable().At(index)
	}
	return dic.FunctionTable().At(0) // return empty function if index is out of bounds
}

func GetStartOfWeekFromTime(t time.Time) uint32 {
	return uint32(t.Truncate(time.Hour * 24 * 7).Unix())
}

// PopulateResourceData flattens resource, scope and profile attributes into a
// single string map.
func PopulateResourceData(dic pprofile.ProfilesDictionary, resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile) (map[string]string, error) {
	numAttrs := resource.Attributes().Len() + scope.Attributes().Len() + profile.AttributeIndices().Len()
	if numAttrs == 0 {
		return map[string]string{}, nil
	}
	attrs := make(map[string]string, numAttrs)

	addAttributes(attrs, resource.Attributes())
	addAttributes(attrs, scope.Attributes())
	profileAttrs, err := pprofile.FromAttributeIndices(dic.AttributeTable(), profile, dic)
	if err != nil {
		return nil, err
	}
	addAttributes(attrs, profileAttrs)

	return attrs, nil
}

func addAttributes(data map[string]string, attrs pcommon.Map) {
	for k, v := range attrs.All() {
		data[k] = v.AsString()
	}
}

func int64ToBytes(value int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(value))
	return buf
}
