// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/internal/metadata"
)

func TestParseJobAndInstanceFeatureGate(t *testing.T) {
	// Enable feature gate
	err := featuregate.GlobalRegistry().Set(metadata.ReceiverPrometheusRemoteWriteDisableJobSplitFeatureGate.ID(), true)
	require.NoError(t, err)

	dest := pcommon.NewMap()
	parseJobAndInstance(dest, "namespace/service", "instance-1")

	expected := pcommon.NewMap()
	expected.PutStr("service.instance.id", "instance-1")
	expected.PutStr("service.name", "namespace/service")

	assert.Equal(t, expected, dest)

	// Disable feature gate (restore)
	err = featuregate.GlobalRegistry().Set(metadata.ReceiverPrometheusRemoteWriteDisableJobSplitFeatureGate.ID(), false)
	require.NoError(t, err)
}
