// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package hostmetricsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootPathNotAllowedOnOS(t *testing.T) {
	assert.NotNil(t, validateRootPath("testdata"))
}

func TestRootPathUnset(t *testing.T) {
	assert.Nil(t, validateRootPath(""))
}
