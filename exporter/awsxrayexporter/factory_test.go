// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayexporter

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			NumberOfWorkers:       8,
			Endpoint:              "",
			RequestTimeoutSeconds: 30,
			MaxRetries:            2,
			NoVerifySSL:           false,
			ProxyAddress:          "",
			Region:                "",
			LocalMode:             false,
			ResourceARN:           "",
			RoleARN:               "",
		},
		skipTimestampValidation: true,
	}, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateDefaultConfigWithSkipTimestampValidation(t *testing.T) {
	factory := NewFactory()

	err := featuregate.GlobalRegistry().Set("exporter.awsxray.skiptimestampvalidation", true)
	assert.NoError(t, err)

	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			NumberOfWorkers:       8,
			Endpoint:              "",
			RequestTimeoutSeconds: 30,
			MaxRetries:            2,
			NoVerifySSL:           false,
			ProxyAddress:          "",
			Region:                "",
			LocalMode:             false,
			ResourceARN:           "",
			RoleARN:               "",
		},
		skipTimestampValidation: true,
	}, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	err = featuregate.GlobalRegistry().Set("exporter.awsxray.skiptimestampvalidation", false)
	assert.NoError(t, err)
}

func TestCreateTraces(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "customname").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	ctx := context.Background()
	exporter, err := factory.CreateTraces(ctx, exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exporter)
}

func TestCreateMetrics(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "customname").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	ctx := context.Background()
	exporter, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(), cfg)
	assert.Error(t, err)
	assert.Nil(t, exporter)
}
