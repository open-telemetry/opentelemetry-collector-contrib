// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

func TestNfsScraper(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{}

	scraper, err := factory.CreateMetrics(context.Background(), scrapertest.NewNopSettings(metadata.Type), cfg)

	if supportedOS {
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
	} else {
		assert.ErrorIs(t, err, errUnsupportedOS)
		assert.Nil(t, scraper)
	}
}
