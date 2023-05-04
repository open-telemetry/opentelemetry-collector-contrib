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

package maps

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeStringMaps(t *testing.T) {
	m1 := map[string]string{
		"key-1": "val-1",
	}

	m2 := map[string]string{
		"key-2": "val-2",
	}

	actual := MergeStringMaps(m1, m2)
	expected := map[string]string{
		"key-1": "val-1",
		"key-2": "val-2",
	}

	require.Equal(t, expected, actual)
}

func TestCloneStringMap(t *testing.T) {
	m := map[string]string{
		"key-1": "val-1",
	}

	actual := CloneStringMap(m)
	expected := map[string]string{
		"key-1": "val-1",
	}

	require.Equal(t, expected, actual)
}
