// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilesample // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilesample"

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

func TestPathGetSetter(t *testing.T) {
	tsNow := time.Now().UTC()
	tests := []struct {
		path string
		val  any
		keys []ottl.Key[*profileSampleContext]
	}{
		{
			path: "values",
			val:  []int64{73, 74, 75},
		},
		{
			path: "attribute_indices",
			val:  []int64{97, 98, 99},
		},
		{
			path: "link_index",
			val:  int64(44),
		},
		{
			path: "timestamps_unix_nano",
			val:  []int64{tsNow.Unix(), 2, 3},
		},
		{
			path: "timestamps",
			val:  []time.Time{tsNow},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			pathParts := strings.Split(tt.path, " ")
			path := &pathtest.Path[*profileSampleContext]{N: pathParts[0]}
			if tt.keys != nil {
				path.KeySlice = tt.keys
			}
			if len(pathParts) > 1 {
				path.NextPath = &pathtest.Path[*profileSampleContext]{N: pathParts[1]}
			}

			sample := pprofile.NewSample()
			dictionary := pprofile.NewProfilesDictionary()

			accessor, err := PathGetSetter(path)
			require.NoError(t, err)

			err = accessor.Set(t.Context(), newProfileSampleContext(sample, dictionary), tt.val)
			require.NoError(t, err)

			got, err := accessor.Get(t.Context(), newProfileSampleContext(sample, dictionary))
			require.NoError(t, err)

			assert.Equal(t, tt.val, got)
		})
	}
}

type profileSampleContext struct {
	sample     pprofile.Sample
	dictionary pprofile.ProfilesDictionary
}

func (p *profileSampleContext) GetProfilesDictionary() pprofile.ProfilesDictionary {
	return p.dictionary
}

func (p *profileSampleContext) GetProfileSample() pprofile.Sample {
	return p.sample
}

func (p *profileSampleContext) AttributeIndices() pcommon.Int32Slice {
	return p.sample.AttributeIndices()
}

func newProfileSampleContext(sample pprofile.Sample, dictionary pprofile.ProfilesDictionary) *profileSampleContext {
	return &profileSampleContext{
		sample:     sample,
		dictionary: dictionary,
	}
}
