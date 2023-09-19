// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
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
		{goarch: "386", hostArch: conventions.AttributeHostArchX86},
		{goarch: "amd64", hostArch: conventions.AttributeHostArchAMD64},
		{goarch: "arm", hostArch: conventions.AttributeHostArchARM32},
		{goarch: "arm64", hostArch: conventions.AttributeHostArchARM64},
		{goarch: "ppc64", hostArch: conventions.AttributeHostArchPPC64},
		{goarch: "ppc64le", hostArch: conventions.AttributeHostArchPPC64},
		{goarch: "s390x", hostArch: conventions.AttributeHostArchS390x},

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
