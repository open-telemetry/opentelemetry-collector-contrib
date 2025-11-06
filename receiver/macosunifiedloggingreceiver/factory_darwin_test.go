// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateDefaultConfig(t *testing.T) {
	expectedCfg := &Config{
		MaxPollInterval: 30 * time.Second,
		MaxLogAge:       24 * time.Hour,
		Format:          "default",
	}

	componentCfg := createDefaultConfig()
	actualCfg, ok := componentCfg.(*Config)
	require.True(t, ok)
	require.Equal(t, expectedCfg, actualCfg)
}
