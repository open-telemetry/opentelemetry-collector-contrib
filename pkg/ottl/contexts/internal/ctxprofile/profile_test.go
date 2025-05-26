// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofile // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofile"
import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

func TestPathGetSetter(t *testing.T) {
	// create tests
	tests := []struct {
		path     string
		val      any
		setFails bool
	}{
		{
			path: "sample_type",
			val:  createValueTypeSlice(),
		},
		{
			path: "sample",
			val:  createSampleSlice(),
		},
		{
			path: "mapping_table",
			val:  createMappingSlice(),
		},
		{
			path: "location_table",
			val:  createLocationSlice(),
		},
		{
			path: "location_indices",
			val:  []int64{5},
		},
		{
			path:     "location_indices error",
			val:      []string{"x"},
			setFails: true,
		},
		{
			path: "function_table",
			val:  createFunctionSlice(),
		},
		{
			path: "attribute_table",
			val:  createAttributeTableSlice(),
		},
		{
			path: "attribute_units",
			val:  createAttributeUnitSlice(),
		},
		{
			path: "link_table",
			val:  createLinkSlice(),
		},
		{
			path: "string_table",
			val:  []string{"", "string"},
		},
		{
			path: "time_unix_nano",
			val:  int64(123),
		},
		{
			path: "time",
			val:  time.Now().UTC(),
		},
		{
			path: "duration_unix_nano",
			val:  int64(10000),
		},
		{
			path: "duration",
			val:  time.Now().UTC(),
		},
		{
			path: "period_type",
			val:  createValueType(),
		},
		{
			path: "period",
			val:  int64(234),
		},
		{
			path: "comment_string_indices",
			val:  []int64{345},
		},
		{
			path: "default_sample_type_string_index",
			val:  int64(456),
		},
		{
			path: "profile_id",
			val:  createProfileID(),
		},
		{
			path:     "profile_id",
			val:      pprofile.NewProfileIDEmpty(),
			setFails: true,
		},
		{
			path: "profile_id string",
			val:  createProfileID().String(),
		},
		{
			path: "profile_id string",
			val: func() string {
				id := pprofile.NewProfileIDEmpty()
				return hex.EncodeToString(id[:])
			}(),
			setFails: true,
		},
		{
			path: "attribute_indices",
			val:  []int64{567},
		},
		{
			path: "dropped_attributes_count",
			val:  int64(678),
		},
		{
			path: "original_payload_format",
			val:  "orgPayloadFormat",
		},
		{
			path: "original_payload",
			val:  []byte{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			pathParts := strings.Split(tt.path, " ")
			path := &pathtest.Path[*profileContext]{N: pathParts[0]}
			if len(pathParts) > 1 {
				path.NextPath = &pathtest.Path[*profileContext]{N: pathParts[1]}
			}

			profile := pprofile.NewProfile()

			accessor, err := PathGetSetter[*profileContext](path)
			require.NoError(t, err)

			err = accessor.Set(context.Background(), newProfileContext(profile), tt.val)
			if tt.setFails {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			got, err := accessor.Get(context.Background(), newProfileContext(profile))
			require.NoError(t, err)

			assert.Equal(t, tt.val, got)
		})
	}
}

type profileContext struct {
	profile pprofile.Profile
}

func (p *profileContext) GetProfile() pprofile.Profile {
	return p.profile
}

func newProfileContext(profile pprofile.Profile) *profileContext {
	return &profileContext{profile: profile}
}

func createValueTypeSlice() pprofile.ValueTypeSlice {
	sl := pprofile.NewValueTypeSlice()
	vt := sl.AppendEmpty()
	vt.CopyTo(createValueType())
	return sl
}

func createValueType() pprofile.ValueType {
	vt := pprofile.NewValueType()
	vt.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
	vt.SetTypeStrindex(2)
	vt.SetUnitStrindex(3)
	return vt
}

func createSampleSlice() pprofile.SampleSlice {
	sl := pprofile.NewSampleSlice()
	sample := sl.AppendEmpty()
	sample.CopyTo(createSample())
	return sl
}

func createMappingSlice() pprofile.MappingSlice {
	sl := pprofile.NewMappingSlice()
	mapping := sl.AppendEmpty()
	mapping.CopyTo(createMapping())
	return sl
}

func createMapping() pprofile.Mapping {
	mapping := pprofile.NewMapping()
	mapping.SetFilenameStrindex(2)
	mapping.SetFileOffset(1)
	mapping.SetHasFilenames(true)
	mapping.SetHasFunctions(true)
	mapping.SetHasInlineFrames(true)
	mapping.SetHasLineNumbers(true)
	mapping.SetMemoryLimit(3)
	mapping.SetMemoryStart(4)
	return mapping
}

func createLocationSlice() pprofile.LocationSlice {
	sl := pprofile.NewLocationSlice()
	location := sl.AppendEmpty()
	location.CopyTo(createLocation())
	return sl
}

func createLocation() pprofile.Location {
	location := pprofile.NewLocation()
	location.SetAddress(1)
	location.SetIsFolded(true)
	location.SetMappingIndex(2)
	return location
}

func createFunctionSlice() pprofile.FunctionSlice {
	sl := pprofile.NewFunctionSlice()
	function := sl.AppendEmpty()
	function.CopyTo(createFunction())
	return sl
}

func createFunction() pprofile.Function {
	function := pprofile.NewFunction()
	function.SetFilenameStrindex(1)
	function.SetNameStrindex(2)
	function.SetStartLine(3)
	function.SetSystemNameStrindex(4)
	return function
}

func createAttributeTableSlice() pprofile.AttributeTableSlice {
	sl := pprofile.NewAttributeTableSlice()
	attribute := sl.AppendEmpty()
	attribute.CopyTo(createAttributeTable())
	return sl
}

func createAttributeTable() pprofile.Attribute {
	attribute := pprofile.NewAttribute()
	attribute.SetKey("key")
	attribute.Value().SetStr("value")
	return attribute
}

func createAttributeUnitSlice() pprofile.AttributeUnitSlice {
	sl := pprofile.NewAttributeUnitSlice()
	attributeUnit := sl.AppendEmpty()
	attributeUnit.CopyTo(createAttributeUnit())
	return sl
}

func createAttributeUnit() pprofile.AttributeUnit {
	attributeUnit := pprofile.NewAttributeUnit()
	attributeUnit.SetUnitStrindex(1)
	attributeUnit.SetAttributeKeyStrindex(2)
	return attributeUnit
}

func createLinkSlice() pprofile.LinkSlice {
	sl := pprofile.NewLinkSlice()
	link := sl.AppendEmpty()
	link.CopyTo(createLink())
	return sl
}

func createLink() pprofile.Link {
	link := pprofile.NewLink()
	link.SetSpanID(pcommon.SpanID([]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	link.SetTraceID(pcommon.TraceID([]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}))
	return link
}

func createProfileID() pprofile.ProfileID {
	return [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
}

func createSample() pprofile.Sample {
	sample := pprofile.NewSample()
	sample.AttributeIndices().Append(1)
	sample.SetLocationsLength(2)
	sample.SetLocationsStartIndex(3)
	sample.TimestampsUnixNano().Append(4)
	sample.Value().Append(5)
	return sample
}
