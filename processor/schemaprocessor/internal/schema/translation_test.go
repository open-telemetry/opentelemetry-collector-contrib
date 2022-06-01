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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslation_SupportedVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario  string
		schema    Schema
		version   *Version
		supported bool
	}{
		{
			scenario:  "Known supported version",
			schema:    newTranslation(&Version{1, 2, 1}),
			version:   &Version{1, 0, 0},
			supported: true,
		},
		{
			scenario:  "Unsupported version",
			schema:    newTranslation(&Version{1, 0, 0}),
			version:   &Version{1, 33, 7},
			supported: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			assert.Equal(
				t,
				tc.supported,
				tc.schema.SupportedVersion(tc.version),
				"Must match the expected supported version",
			)
		})
	}
}
