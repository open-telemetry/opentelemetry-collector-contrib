// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package dockerstatsreceiver

import (
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on GH runners: until flakiness is investigated")
	}

	// Start a docker container to ensure container metrics are available
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image: "alpine:latest",
				Name:  "dockerstatsreceiver-test",
			},
		),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceAttributeValue("container.hostname"),
			pmetrictest.IgnoreResourceAttributeValue("container.id"),
			pmetrictest.IgnoreResourceAttributeValue("container.image.name"),
			pmetrictest.IgnoreResourceAttributeValue("container.name"),
			pmetrictest.IgnoreResourceAttributeValue("container.runtime"),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()),
	).Run(t)
}
