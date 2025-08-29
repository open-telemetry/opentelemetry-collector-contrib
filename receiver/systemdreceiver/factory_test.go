// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()

	scraper, err := factory.CreateMetrics(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		factory.CreateDefaultConfig(),
		nil,
	)

	if runtime.GOOS == "linux" {
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
	} else {
		assert.Error(t, err)
		assert.Equal(t, errNonLinux, err)
		assert.Nil(t, scraper)
	}
}
