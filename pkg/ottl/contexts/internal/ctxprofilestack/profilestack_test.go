// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxprofilestack // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxprofilestack"

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
	tests := []struct {
		path        string
		getSetError bool
		val         any
		keys        []ottl.Key[*profileStackContext]
	}{
		{
			path: "location_indices",
			val:  []int64{97, 98, 99},
		},
		{
			path:        "invalid_path",
			getSetError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			pathParts := strings.Split(tt.path, " ")
			path := &pathtest.Path[*profileStackContext]{N: pathParts[0]}
			if tt.keys != nil {
				path.KeySlice = tt.keys
			}
			if len(pathParts) > 1 {
				path.NextPath = &pathtest.Path[*profileStackContext]{N: pathParts[1]}
			}

			stack := pprofile.NewStack()
			dictionary := pprofile.NewProfilesDictionary()

			accessor, err := PathGetSetter(path)
			if tt.getSetError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			err = accessor.Set(t.Context(), newProfileStackContext(stack, dictionary), tt.val)
			require.NoError(t, err)

			got, err := accessor.Get(t.Context(), newProfileStackContext(stack, dictionary))
			require.NoError(t, err)
			assert.Equal(t, tt.val, got)
		})
	}
}

type profileStackContext struct {
	stack      pprofile.Stack
	dictionary pprofile.ProfilesDictionary
}

func (p *profileStackContext) GetProfilesDictionary() pprofile.ProfilesDictionary {
	return p.dictionary
}

func (p *profileStackContext) GetProfileStack() pprofile.Stack {
	return p.stack
}

func newProfileStackContext(stack pprofile.Stack, dictionary pprofile.ProfilesDictionary) *profileStackContext {
	return &profileStackContext{
		stack:      stack,
		dictionary: dictionary,
	}
}
