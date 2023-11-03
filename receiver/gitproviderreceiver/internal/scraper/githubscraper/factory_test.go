// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

var creationSet = receivertest.NewNopCreateSettings()

func TestCreateDefaultConfig(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg, "failed to create default config")
}

func TestCreateMetricsScraper(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()

	mReceiver, err := factory.CreateMetricsScraper(context.Background(), creationSet, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, mReceiver)
}
