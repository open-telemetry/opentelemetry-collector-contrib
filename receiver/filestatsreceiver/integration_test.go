// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package filestatsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_Integration(t *testing.T) {
	expectedFile := filepath.Join("testdata", "integration", "expected.yaml")

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				rCfg.Include = "filestats_*"
			}),
		scraperinttest.WithExpectedFile(expectedFile),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)

}
