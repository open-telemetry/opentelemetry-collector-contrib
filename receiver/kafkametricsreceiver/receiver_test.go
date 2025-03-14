// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestNewReceiver(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers", "topics", "consumers"}
	r, err := newMetricsReceiver(
		*c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop(),
		mockNewSaramaClient, mockNewClusterAdmin,
	)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewReceiver_InvalidScraper(t *testing.T) {
	c := createDefaultConfig().(*Config)
	c.Scrapers = []string{"brokers", "cpu"}

	r, err := newMetricsReceiver(
		*c, receivertest.NewNopSettings(metadata.Type), consumertest.NewNop(),
		mockNewSaramaClient, mockNewClusterAdmin,
	)
	assert.EqualError(t, err, `unknown scraper type "cpu"`)
	assert.Nil(t, r)
}
