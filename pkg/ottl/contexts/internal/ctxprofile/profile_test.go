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
			path: "location_indices",
			val:  []int64{5},
		},
		{
			path:     "location_indices error",
			val:      []string{"x"},
			setFails: true,
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
			path: "default_sample_type_index",
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
