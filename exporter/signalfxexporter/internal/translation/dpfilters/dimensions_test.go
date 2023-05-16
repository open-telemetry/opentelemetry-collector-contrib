// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dpfilters

import (
	"testing"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/require"
)

func TestDimensionsFilter(t *testing.T) {
	tests := []struct {
		name        string
		filter      map[string][]string
		input       []*sfxpb.Dimension
		shouldMatch bool
		shouldError bool
	}{
		{
			name:        "Empty filter does not empty slice of dimensions",
			filter:      map[string][]string{},
			input:       []*sfxpb.Dimension{},
			shouldMatch: false,
		},
		{
			name: "Non-empty filter does not match empty slice of dimensions",
			filter: map[string][]string{
				"app": {"test"},
			},
			input:       []*sfxpb.Dimension{},
			shouldMatch: false,
		},
		{
			name: "Filter does not match different dimension",
			filter: map[string][]string{
				"app": {"test"},
			},
			input: []*sfxpb.Dimension{
				{
					Key:   "version",
					Value: "latest",
				},
			},
			shouldMatch: false,
		},
		{
			name: "Filter matches on exact match of a dimension",
			filter: map[string][]string{
				"app":     {"test"},
				"version": {"*"},
			},
			input: []*sfxpb.Dimension{
				{
					Key:   "app",
					Value: "test",
				},
			},
			shouldMatch: true,
		},
		{
			name: "Filter matches on exact match with multiple dimensions in input slice",
			filter: map[string][]string{
				"app": {"test"},
			},
			input: []*sfxpb.Dimension{
				{
					Key:   "app",
					Value: "test",
				},
				{
					Key:   "version",
					Value: "2.0",
				},
			},
			shouldMatch: true,
		},
		{
			name: "Filter matches on regex with multiple dimensions in input slice",
			filter: map[string][]string{
				"version": {`/\d+\.\d+/`},
			},
			input: []*sfxpb.Dimension{
				{
					Key:   "app",
					Value: "test",
				},
				{
					Key:   "version",
					Value: "2.0",
				},
			},
			shouldMatch: true,
		},
		{
			name: "Filter does not match on regex",
			filter: map[string][]string{
				"version": {`/\d+\.\d+/`},
			},
			input: []*sfxpb.Dimension{
				{
					Key:   "app",
					Value: "test",
				},
				{
					Key:   "version",
					Value: "bad",
				},
			},
			shouldMatch: false,
		},
		{
			name: "Error creating filter with no dimension values",
			filter: map[string][]string{
				"version": {},
			},
			shouldError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := newDimensionsFilter(test.filter)
			if test.shouldError {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}

			require.Equal(t, test.shouldMatch, f.Matches(test.input))
		})
	}
}
