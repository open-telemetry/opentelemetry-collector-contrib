// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package gopsutilenv

import (
	"context"
	"testing"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/stretchr/testify/assert"
)

func TestRootPathNotAllowedOnOS(t *testing.T) {
	assert.Error(t, ValidateRootPath("testdata"))
}

func TestRootPathUnset(t *testing.T) {
	assert.NoError(t, ValidateRootPath(""))
}

func TestGetEnvWithContext(t *testing.T) {
	val := GetEnvWithContext(context.Background(), string(common.HostProcEnvKey), "default")
	assert.Equal(t, "default", val)
}
