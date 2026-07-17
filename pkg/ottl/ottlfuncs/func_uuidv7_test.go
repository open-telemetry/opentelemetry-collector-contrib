// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_UUIDv7(t *testing.T) {
	exprFunc, err := uuidV7[any]()
	require.NoError(t, err)

	value, err := exprFunc(nil, nil)
	require.NoError(t, err)

	uuidStr, ok := value.(string)
	require.True(t, ok)

	assert.Len(t, uuidStr, 36)

	assert.Equal(t, "7", string(uuidStr[14]), "UUID version must be 7")

	assert.Contains(t, []string{"8", "9", "a", "b"}, string(uuidStr[19]), "UUID variant must be RFC 4122")
}

func Test_UUIDv7_uniqueness(t *testing.T) {
	exprFunc, err := uuidV7[any]()
	require.NoError(t, err)

	v1, _ := exprFunc(nil, nil)
	v2, _ := exprFunc(nil, nil)

	assert.NotEqual(t, v1, v2, "Successive UUIDv7 calls must produce unique values")
}
