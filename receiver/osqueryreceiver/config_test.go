// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestConfig_Validate(t *testing.T) {
	cfg := createDefaultConfig()
	rc := cfg.(*Config)
	assert.Error(t, rc.Validate())

	rc.Queries = []string{"select * from certificates"}
	assert.NoError(t, rc.Validate())
}

func TestConfig_Validate_Collections(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Collections = []string{"system_info"}
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_InvalidCollection(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Collections = []string{"not_a_real_collection"}
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_QueriesAndCollections(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queries = []string{"select * from certificates"}
	cfg.Collections = []string{"package_info"}
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_NegativeSnapshotInterval(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Collections = []string{"system_info"}
	cfg.SnapshotInterval = -time.Second
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_SnapshotIntervalAndStorage(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Collections = []string{"system_info"}
	cfg.SnapshotInterval = time.Hour
	id := component.MustNewID("file_storage")
	cfg.StorageID = &id
	assert.NoError(t, cfg.Validate())
}
