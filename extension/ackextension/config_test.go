// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateConfigWithEmptyOrZeroValConfig(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	require.Equal(t, err, nil)
	require.Equal(t, cfg.MaxNumPendingAcksPerPartition, defaultMaxNumPendingAcksPerPartition)
	require.Equal(t, cfg.MaxNumPartition, defaultMaxNumPartition)
}

func TestValidateConfigWithValidConfig(t *testing.T) {
	cfg := Config{
		MaxNumPendingAcksPerPartition: 3,
		MaxNumPartition:               5,
	}
	err := cfg.Validate()
	require.Equal(t, err, nil)
	require.Equal(t, cfg.MaxNumPendingAcksPerPartition, 3)
	require.Equal(t, cfg.MaxNumPartition, 5)
}
