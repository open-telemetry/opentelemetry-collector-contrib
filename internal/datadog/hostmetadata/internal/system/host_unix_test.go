// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build !windows

package system

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSystemFQDN(t *testing.T) {
	hostnamePath = "/nonexistent"
	_, err := getSystemFQDN(context.Background())

	// Fail silently when not available
	require.NoError(t, err)
}
