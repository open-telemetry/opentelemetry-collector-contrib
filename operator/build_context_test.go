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

package operator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildContext(t *testing.T) {
	t.Run("PrependNamespace", func(t *testing.T) {
		bc := BuildContext{
			Namespace: "$.test",
		}

		t.Run("Standard", func(t *testing.T) {
			id := bc.PrependNamespace("testid")
			require.Equal(t, "$.test.testid", id)
		})

		t.Run("AlreadyPrefixed", func(t *testing.T) {
			id := bc.PrependNamespace("$.myns.testid")
			require.Equal(t, "$.myns.testid", id)
		})
	})

	t.Run("WithSubNamespace", func(t *testing.T) {
		bc := BuildContext{
			Namespace: "$.ns",
		}
		bc2 := bc.WithSubNamespace("subns")
		require.Equal(t, "$.ns.subns", bc2.Namespace)
		require.Equal(t, "$.ns", bc.Namespace)
	})

	t.Run("WithDefaultOutputIDs", func(t *testing.T) {
		bc := BuildContext{
			DefaultOutputIDs: []string{"orig"},
		}
		bc2 := bc.WithDefaultOutputIDs([]string{"id1", "id2"})
		require.Equal(t, []string{"id1", "id2"}, bc2.DefaultOutputIDs)
		require.Equal(t, []string{"orig"}, bc.DefaultOutputIDs)

	})
}
