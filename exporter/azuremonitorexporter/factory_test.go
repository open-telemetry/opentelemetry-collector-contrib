// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter/internal/metadata"
)

// An inappropriate config
type badConfig struct{}

func TestCreateTracesUsingSpecificTransportChannel(t *testing.T) {
	// mock transport channel creation
	f := &factory{}
	ctx := context.Background()
	params := exportertest.NewNopSettings(metadata.Type)
	config := createDefaultConfig().(*Config)
	config.ConnectionString = "InstrumentationKey=test-key;IngestionEndpoint=https://test-endpoint/"
	exporter, err := f.createTracesExporter(ctx, params, config)
	assert.NotNil(t, exporter)
	assert.NoError(t, err)
}

func TestCreateTracesUsingDefaultTransportChannel(t *testing.T) {
	// We get the default transport channel creation, if we don't specify one during f creation
	f := factory{}
	ctx := context.Background()
	config := createDefaultConfig().(*Config)
	config.ConnectionString = "InstrumentationKey=test-key;IngestionEndpoint=https://test-endpoint/"
	exporter, err := f.createTracesExporter(ctx, exportertest.NewNopSettings(metadata.Type), config)
	assert.NotNil(t, exporter)
	assert.NoError(t, err)
}

func TestCreateTracesUsingBadConfig(t *testing.T) {
	// We get the default transport channel creation, if we don't specify one during factory creation
	f := factory{}
	ctx := context.Background()
	params := exportertest.NewNopSettings(metadata.Type)

	badConfig := &badConfig{}

	exporter, err := f.createTracesExporter(ctx, params, badConfig)
	assert.Nil(t, exporter)
	assert.Error(t, err)
}
