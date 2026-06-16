// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestNewExtension(t *testing.T) {
	// test
	cfg := testConfig()
	e := newExtension(cfg, componenttest.NewNopTelemetrySettings())

	// verify
	assert.NotNil(t, e)
}

func testConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = "127.0.0.1:5778"
	return cfg
}
