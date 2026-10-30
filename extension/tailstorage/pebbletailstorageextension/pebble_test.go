// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package pebbletailstorageextension

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewPebbleDBOptions(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{name: "explicit sizes", cfg: &Config{CacheSizeMiB: 2, MemTableSizeMiB: 1}},
		{name: "default sizes", cfg: &Config{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, created, err := newPebbleDB(t.TempDir(), tt.cfg, zap.NewNop())
			require.NoError(t, err)
			assert.True(t, created)

			require.NoError(t, db.Set([]byte("k"), []byte("v"), pebble.NoSync))
			got, closer, err := db.Get([]byte("k"))
			require.NoError(t, err)
			assert.Equal(t, []byte("v"), got)
			require.NoError(t, closer.Close())

			require.NoError(t, db.Close())
		})
	}
}
