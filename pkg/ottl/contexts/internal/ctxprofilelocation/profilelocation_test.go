// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilelocation // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilelocation"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

func TestPathGetSetter(t *testing.T) {
	lineSlice := pprofile.NewLineSlice()
	for _, lineValue := range []int64{73, 74, 75} {
		line := lineSlice.AppendEmpty()
		line.SetLine(lineValue)
	}
	tests := []struct {
		path string
		val  any
		keys []ottl.Key[*profileLocationContext]
	}{
		{
			path: "mapping_index",
			val:  int64(42),
		},
		{
			path: "address",
			val:  uint64(43),
		},
		{
			path: "line",
			val:  lineSlice,
		},
		{
			path: "attribute_indices",
			val:  []int64{97, 98, 99},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			pathParts := strings.Split(tt.path, " ")
			path := &pathtest.Path[*profileLocationContext]{N: pathParts[0]}
			if tt.keys != nil {
				path.KeySlice = tt.keys
			}
			if len(pathParts) > 1 {
				path.NextPath = &pathtest.Path[*profileLocationContext]{N: pathParts[1]}
			}

			location := pprofile.NewLocation()
			dictionary := pprofile.NewProfilesDictionary()

			accessor, err := PathGetSetter(path)
			require.NoError(t, err)

			err = accessor.Set(t.Context(), newProfileLocationContext(location, dictionary), tt.val)
			require.NoError(t, err)

			got, err := accessor.Get(t.Context(), newProfileLocationContext(location, dictionary))
			require.NoError(t, err)

			assert.Equal(t, tt.val, got)
		})
	}
}

type profileLocationContext struct {
	location   pprofile.Location
	dictionary pprofile.ProfilesDictionary
}

func (p *profileLocationContext) GetProfilesDictionary() pprofile.ProfilesDictionary {
	return p.dictionary
}

func (p *profileLocationContext) GetProfileLocation() pprofile.Location {
	return p.location
}

func newProfileLocationContext(location pprofile.Location, dictionary pprofile.ProfilesDictionary) *profileLocationContext {
	return &profileLocationContext{location: location, dictionary: dictionary}
}
