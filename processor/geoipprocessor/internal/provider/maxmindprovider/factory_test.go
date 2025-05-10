// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maxmind

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateProvider(t *testing.T) {
	factory := &Factory{}
	cfg := &Config{
		DatabasePath: "",
	}

	provider, err := factory.CreateGeoIPProvider(context.Background(), processortest.NewNopSettings(), cfg)

	assert.ErrorContains(t, err, "could not open geoip database")
	assert.Nil(t, provider)
}
