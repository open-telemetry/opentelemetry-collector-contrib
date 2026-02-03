// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonfileobserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/jsonfileobserver/internal/metadata"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	require.True(t, ok)

	assert.Equal(t, "", config.Path)
	assert.Equal(t, 10*time.Second, config.RefreshInterval)
}

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}
