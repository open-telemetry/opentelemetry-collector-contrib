// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	receiver := cfg.Receivers[component.NewID(metadata.Type)]
	assert.Equal(t, factory.CreateDefaultConfig(), receiver)

	receiver = cfg.Receivers[component.NewIDWithName(metadata.Type, "2")].(*Config)
	assert.NoError(t, componenttest.CheckConfigStruct(receiver))
	assert.Equal(
		t,
		&Config{
			ConnectionString: goodConnectionString,
			Logs:             LogsConfig{ContainerName: logsContainerName},
			Traces:           TracesConfig{ContainerName: tracesContainerName},
		},
		receiver)
}
