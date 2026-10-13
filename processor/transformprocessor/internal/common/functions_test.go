// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceFunctions(t *testing.T) {
	funcs := ResourceFunctions()
	assert.NotEmpty(t, funcs)
	assert.Contains(t, funcs, "set")
}

func TestScopeFunctions(t *testing.T) {
	funcs := ScopeFunctions()
	assert.NotEmpty(t, funcs)
	assert.Contains(t, funcs, "set")
}
