// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_UUIDv7(t *testing.T) {
	exprFunc, err := uuid[any]()
	assert.NoError(t, err)

	value, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.IsType(t, "", value)
	assert.NotEmpty(t, value)
}
