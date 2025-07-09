// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package hostmetricsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
)

func TestRootPathNotAllowedOnOS(t *testing.T) {
	assert.Error(t, gopsutilenv.ValidateRootPath("testdata"))
}

func TestRootPathUnset(t *testing.T) {
	assert.NoError(t, gopsutilenv.ValidateRootPath(""))
}
