// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build !windows
// +build !windows

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSystemFQDN(t *testing.T) {
	hostnamePath = "/nonexistent"
	_, err := getSystemFQDN()

	// Fail silently when not available
	require.NoError(t, err)
}
