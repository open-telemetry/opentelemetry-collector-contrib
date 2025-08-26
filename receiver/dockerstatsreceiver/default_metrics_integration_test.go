// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package dockerstatsreceiver

import (
	"os"
	"runtime"
	"testing"

	"github.com/testcontainers/testcontainers-go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	if runtime.GOOS == "darwin" && os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on Darwin GH runners: test requires Docker service")
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
