// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package icmpcheckreceiver

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = 100 * time.Millisecond
				rCfg.Targets = []PingTarget{
					{Host: "github.com", PingCount: 4},
				}
			},
		),
		scraperinttest.WithCompareTimeout(30*time.Second),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues("ping.rtt.max", "ping.rtt.min", "ping.rtt.avg"),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()),
	).Run(t)
}
