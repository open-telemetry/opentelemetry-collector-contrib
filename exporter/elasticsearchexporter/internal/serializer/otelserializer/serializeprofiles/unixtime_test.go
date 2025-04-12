// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnixTime64_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		time unixTime64
		want string
	}{
		{
			name: "zero",
			time: newUnixTime64(0),
			want: `"1970-01-01T00:00:00Z"`,
		},
		{
			name: "32bit value (assure there is no regression)",
			time: newUnixTime64(1710349106),
			want: `"2024-03-13T16:58:26Z"`,
		},
		{
			name: "64bit value",
			time: newUnixTime64(1710349106864964685),
			want: `"2024-03-13T16:58:26.864964685Z"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := test.time.MarshalJSON()
			require.NoError(t, err)
			assert.Equal(t, test.want, string(b))
		})
	}
}
