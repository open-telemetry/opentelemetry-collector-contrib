// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		input    string
		err      error
		ident    *Identifier
	}{
		{
			scenario: "valid schema identifier",
			input:    "1.33.7",
			err:      nil,
			ident:    &Identifier{Major: 1, Minor: 33, Patch: 7},
		},
		{
			scenario: "schema identifier incomplete",
			input:    "1.0",
			err:      ErrInvalidIdentifier,
			ident:    nil,
		},
		{
			scenario: "schema indentifier with non numerical value",
			input:    "v1.0.0",
			err:      strconv.ErrSyntax,
			ident:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			ident, err := NewIdentifier(tc.input)

			assert.ErrorIs(t, err, tc.err, "MUST have the expected error")
			assert.Equal(t, tc.ident, ident, "MUST match the expected value")
			if ident != nil {
				assert.Equal(t, tc.input, ident.String(), "Stringer value MUST match input value")
			}
		})
	}
}

func TestParsingIdentifierFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		path     string
		ident    *Identifier
		err      error
	}{
		{scenario: "No path set", path: "", ident: nil, err: ErrInvalidIdentifier},
		{scenario: "no schema identifier defined", path: "foo/bar", ident: nil, err: ErrInvalidIdentifier},
		{scenario: "a valid path with schema identifier", path: "foo/bar/1.8.0", ident: &Identifier{Major: 1, Minor: 8, Patch: 0}, err: nil},
		{scenario: "a path with a trailing slash", path: "foo/bar/1.5.3/", ident: nil, err: ErrInvalidIdentifier},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			ident, err := ReadIdentifierFromPath(tc.path)

			assert.ErrorIs(t, err, tc.err, "Must be the expected error when processor")
			assert.Equal(t, tc.ident, ident)
		})
	}
}
