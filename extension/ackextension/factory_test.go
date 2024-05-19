// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ackextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	expectType := component.MustNewType("ack")
	require.Equal(t, expectType, f.Type())

	cfg := f.CreateDefaultConfig().(*Config)
	require.Equal(t, defaultStorageType, cfg.StorageID)
	require.Equal(t, defaultMaxNumPendingAcksPerPartition, cfg.MaxNumPendingAcksPerPartition)
	require.Equal(t, defaultMaxNumPartition, cfg.MaxNumPartition)
}
