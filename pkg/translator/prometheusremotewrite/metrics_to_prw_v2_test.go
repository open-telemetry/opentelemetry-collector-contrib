// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromMetricsV2(t *testing.T) {
	settings := Settings{
		Namespace:           "",
		ExternalLabels:      nil,
		DisableTargetInfo:   false,
		ExportCreatedMetric: false,
		AddMetricSuffixes:   false,
		SendMetadata:        false,
	}

	payload := createExportRequest(5, 0, 1, 3, 0)

	tsMap, err := FromMetricsV2(payload.Metrics(), settings)
	require.NoError(t, err)
	require.NotNil(t, tsMap)
	// TODO add assert equal with expected values
}
