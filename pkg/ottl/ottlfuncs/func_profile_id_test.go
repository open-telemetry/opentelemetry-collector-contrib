// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func Test_profileID(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		want  pprofile.ProfileID
	}{
		{
			name:  "create profile id",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			want:  pprofile.ProfileID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := profileID[any](tt.bytes)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_profileID_validation(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		err   string
	}{
		{
			name:  "nil profile id",
			bytes: nil,
			err:   "profile ids must be 16 bytes",
		},
		{
			name:  "byte slice less than 16",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			err:   "profile ids must be 16 bytes",
		},
		{
			name:  "byte slice longer than 16",
			bytes: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
			err:   "profile ids must be 16 bytes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := profileID[any](tt.bytes)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.err)
		})
	}
}
