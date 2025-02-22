// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg, "failed to create default config")
}

func TestCreateProcessor(t *testing.T) {
	cfg := createDefaultConfig()
	set := processortest.NewNopSettings(metadata.Type)
	tp, err := createTracesProcessor(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err, "cannot create trace processor")
	assert.NotNil(t, tp)
}

func TestCreateProcessorLogs(t *testing.T) {
	cfg := createDefaultConfig()
	set := processortest.NewNopSettings(metadata.Type)
	tp, err := createLogsProcessor(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err, "cannot create logs processor")
	assert.NotNil(t, tp)
}
