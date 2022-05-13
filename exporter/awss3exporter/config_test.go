// Copyright 2021 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      shttp://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awss3exporter

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "default.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[config.NewComponentID(typeStr)]

	assert.Equal(t, e,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),

			S3Uploader: S3UploaderConfig{
				Region:      "us-east-1",
				S3Partition: "minute",
			},
			BatchCount:        1000,
			MetricDescriptors: make([]MetricDescriptor, 0),
		},
	)
}

func TestConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := servicetest.LoadConfigAndValidate(
		filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[config.NewComponentID(typeStr)]

	assert.Equal(t, e,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),

			S3Uploader: S3UploaderConfig{
				Region:      "us-east-1",
				S3Bucket:    "foo",
				S3Prefix:    "bar",
				S3Partition: "minute",
			},
			BatchCount:        1000,
			MetricDescriptors: make([]MetricDescriptor, 0),
		},
	)
}
