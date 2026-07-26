// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/metadata"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, metadata.Type, f.Type())
	cfg := f.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	duration, _ := time.ParseDuration("30s")
	assert.Equal(t, duration, cfg.(*Config).CollectionInterval)
}

func TestCreateLogsReceiver_NoSnapshotInterval_ReturnsPlainController(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queries = []string{"select * from block_devices"}

	recv, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	_, isDual := recv.(*dualIntervalReceiver)
	assert.False(t, isDual, "expected a plain scraperhelper controller when snapshot_interval is unset")
}

func TestCreateLogsReceiver_WithSnapshotInterval_StartsAndStopsBothControllers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Collections = []string{"system_info"}
	cfg.SnapshotInterval = time.Hour

	recv, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	dual, isDual := recv.(*dualIntervalReceiver)
	require.True(t, isDual, "expected a dualIntervalReceiver when snapshot_interval is set and collections are configured")
	assert.NotNil(t, dual.changeOnly)
	assert.NotNil(t, dual.snapshot)

	require.NoError(t, recv.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, recv.Shutdown(t.Context()))
}

func TestCreateLogsReceiver_SnapshotIntervalWithoutCollections_ReturnsPlainController(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queries = []string{"select * from block_devices"}
	cfg.SnapshotInterval = time.Hour

	recv, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	_, isDual := recv.(*dualIntervalReceiver)
	assert.False(t, isDual, "a snapshot ticker with no collections to snapshot is pointless")
}

var _ receiver.Logs = (*dualIntervalReceiver)(nil)
