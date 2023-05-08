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

package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseKeyspace(t *testing.T) {
	ks, err := parseKeyspaceString(9, "keys=1,expires=2,avg_ttl=3")
	require.Nil(t, err)
	require.Equal(t, "9", ks.db)
	require.Equal(t, 1, ks.keys)
	require.Equal(t, 2, ks.expires)
	require.Equal(t, 3, ks.avgTTL)
}

func TestParseMalformedKeyspace(t *testing.T) {
	tests := []struct{ name, keyspace string }{
		{"missing value", "keys=1,expires=2,avg_ttl="},
		{"missing equals", "keys=1,expires=2,avg_ttl"},
		{"unexpected key", "xyz,keys=1,expires=2"},
		{"no usable data", "foo"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseKeyspaceString(0, test.keyspace)
			require.NotNil(t, err)
		})
	}
}
