// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGOOSToOsType(t *testing.T) {
	assert.Equal(t, "darwin", GOOSToOSType("darwin"))
	assert.Equal(t, "linux", GOOSToOSType("linux"))
	assert.Equal(t, "windows", GOOSToOSType("windows"))
	assert.Equal(t, "dragonflybsd", GOOSToOSType("dragonfly"))
	assert.Equal(t, "z_os", GOOSToOSType("zos"))
}
