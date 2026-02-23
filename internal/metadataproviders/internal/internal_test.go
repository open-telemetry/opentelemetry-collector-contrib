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

func TestGOARCHToHostArch(t *testing.T) {
	tests := []struct {
		goarch   string
		hostArch string
	}{
		// well-known values that are supported by Go
		{goarch: "386", hostArch: "x86"},
		{goarch: "amd64", hostArch: "amd64"},
		{goarch: "arm", hostArch: "arm32"},
		{goarch: "arm64", hostArch: "arm64"},
		{goarch: "ppc64", hostArch: "ppc64"},
		{goarch: "ppc64le", hostArch: "ppc64"},
		{goarch: "s390x", hostArch: "s390x"},

		// not well-known values
		{goarch: "mips", hostArch: "mips"},
		{goarch: "mips64", hostArch: "mips64"},
		{goarch: "mips64le", hostArch: "mips64le"},
		{goarch: "mipsle", hostArch: "mipsle"},
		{goarch: "riscv64", hostArch: "riscv64"},
	}

	for _, tt := range tests {
		t.Run(tt.goarch, func(t *testing.T) {
			assert.Equal(t, tt.hostArch, GOARCHtoHostArch(tt.goarch))
		})
	}
}
