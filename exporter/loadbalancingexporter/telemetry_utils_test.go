// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

func getTelemetryAssets(t require.TestingT) (exporter.Settings, *metadata.TelemetryBuilder) {
	s := exportertest.NewNopSettings(metadata.Type)
	tb, err := metadata.NewTelemetryBuilder(s.TelemetrySettings)
	require.NoError(t, err)
	return s, tb
}
