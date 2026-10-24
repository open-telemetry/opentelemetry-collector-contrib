// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisfleettrackerextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension/internal/metadata"
)

func TestFactory_Type(t *testing.T) {
	assert.Equal(t, metadata.Type, NewFactory().Type())
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	assert.Equal(t, "fleet_tracker", cfg.KeyPrefix)
	assert.Equal(t, 3*time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 10*time.Second, cfg.TTL)
	assert.True(t, cfg.TLS.Insecure)
}

func TestFactory_Create_FleetDefaultsToComponentID(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:6379"

	id := component.NewIDWithName(metadata.Type, "prod")
	set := extensiontest.NewNopSettings(metadata.Type)
	set.ID = id

	ext, err := f.Create(t.Context(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, id.String(), cfg.Fleet)
}

func TestFactory_Create_ExplicitFleetIsPreserved(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:6379"
	cfg.Fleet = "shared-fleet"

	ext, err := f.Create(t.Context(), extensiontest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, "shared-fleet", cfg.Fleet)
}
