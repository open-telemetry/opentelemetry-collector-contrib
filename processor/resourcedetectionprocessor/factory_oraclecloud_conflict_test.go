// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build omit_detector_oraclecloud && enable_resourcedetection_oraclecloud_detector

package resourcedetectionprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

func TestUnifiedTagsOmitTakesPrecedence(t *testing.T) {
	assert.Nil(t, detectorFactories()[internal.DetectorType("oraclecloud")])
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Detectors = []string{"oraclecloud"}
	processor, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.ErrorContains(t, err, `detector "oraclecloud" is not compiled into this binary`)
	assert.Nil(t, processor)
}
