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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func TestPathGetSetter(t *testing.T) {
	// create tests
	tests := []struct {
		path     string
		val      any
		keys     []ottl.Key[*profileContext]
		setFails bool
	}{
		{
			path: "sample_type",
			val:  createValueType(),
		},
		{
			path: "sample_type type",
			val:  "cpu",
		},
		{
			path: "sample_type unit",
			val:  "cycles",
		},
		{
			path: "sample",
			val:  createSampleSlice(),
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
			path: "period_type type",
			val:  "heap",
		},
		{
			path: "period_type unit",
			val:  "bytes",
		},
		{
			path: "period",
			val:  int64(234),
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
		{
			path: "attributes",
			val: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("akey", "val")
				return m
			}(),
		},
		{
			path: "attributes",
			keys: []ottl.Key[*profileContext]{
				&pathtest.Key[*profileContext]{
					S: ottltest.Strp("akey"),
				},
			},
			val: "val",
		},
		{
			path: "attributes",
			keys: []ottl.Key[*profileContext]{
				&pathtest.Key[*profileContext]{
					S: ottltest.Strp("akey"),
				},
				&pathtest.Key[*profileContext]{
					S: ottltest.Strp("bkey"),
				},
			},
			val: "val",
		},
		{
			path: "attributes",
			keys: []ottl.Key[*profileContext]{
				&pathtest.Key[*profileContext]{
					G: &ottl.StandardGetSetter[*profileContext]{
						Getter: func(context.Context, *profileContext) (any, error) {
							return "", nil
						},
						Setter: func(context.Context, *profileContext, any) error {
							return nil
						},
					},
				},
			},
			val: "val",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			pathParts := strings.Split(tt.path, " ")
			path := &pathtest.Path[*profileContext]{N: pathParts[0]}
			if tt.keys != nil {
				path.KeySlice = tt.keys
			}
			if len(pathParts) > 1 {
				path.NextPath = &pathtest.Path[*profileContext]{N: pathParts[1]}
			}

			profile := pprofile.NewProfile()
			dictionary := pprofile.NewProfilesDictionary()

			accessor, err := PathGetSetter(path)
			require.NoError(t, err)

			err = accessor.Set(t.Context(), newProfileContext(profile, dictionary), tt.val)
			if tt.setFails {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			got, err := accessor.Get(t.Context(), newProfileContext(profile, dictionary))
			require.NoError(t, err)

			assert.Equal(t, tt.val, got)
		})
	}
}

type profileContext struct {
	profile    pprofile.Profile
	dictionary pprofile.ProfilesDictionary
}

func (p *profileContext) GetProfilesDictionary() pprofile.ProfilesDictionary {
	return p.dictionary
}

func (p *profileContext) GetProfile() pprofile.Profile {
	return p.profile
}

func (p *profileContext) AttributeIndices() pcommon.Int32Slice {
	return p.profile.AttributeIndices()
}

func newProfileContext(profile pprofile.Profile, dictionary pprofile.ProfilesDictionary) *profileContext {
	return &profileContext{
		profile:    profile,
		dictionary: dictionary,
	}
}

func createValueType() pprofile.ValueType {
	vt := pprofile.NewValueType()
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
	sample.TimestampsUnixNano().Append(4)
	sample.Values().Append(5)
	return sample
}
