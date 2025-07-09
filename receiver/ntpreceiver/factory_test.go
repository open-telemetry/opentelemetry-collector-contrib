// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ntpreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateDefaultConfig(t *testing.T) {
	c := createDefaultConfig().(*Config)
	require.Equal(t, 4, c.Version)
	require.Equal(t, "pool.ntp.org:123", c.Endpoint)
	require.Equal(t, 30*time.Minute, c.CollectionInterval)
}
