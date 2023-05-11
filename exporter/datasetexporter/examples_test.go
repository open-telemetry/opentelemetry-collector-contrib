// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/ballastextension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
)

type SuiteExamples struct {
	suite.Suite
}

func (s *SuiteExamples) SetupTest() {
	os.Clearenv()
}

func TestSuiteExamples(t *testing.T) {
	suite.Run(t, new(SuiteExamples))
}

func (s *SuiteExamples) TestLoadAllConfig() {
	factories := newTestComponents(s.T())

	configs := []string{
		filepath.Join("examples", "e2e", "otel-config.yaml"),
	}

	for _, cfg := range configs {
		s.T().Run(cfg, func(t *testing.T) {
			t.Setenv("DATASET_API_KEY", "dateset-api-key")
			errD := os.MkdirAll(filepath.Join("tmp", "otc"), os.ModePerm)
			require.NoError(t, errD)
			_, err := otelcoltest.LoadConfigAndValidate(cfg, factories)
			require.NoError(t, err)
		})
	}
}

func newTestComponents(t *testing.T) otelcol.Factories {
	var (
		factories otelcol.Factories
		err       error
	)
	factories.Receivers, err = receiver.MakeFactoryMap(
		[]receiver.Factory{
			otlpreceiver.NewFactory(),
			filelogreceiver.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Processors, err = processor.MakeFactoryMap(
		[]processor.Factory{
			batchprocessor.NewFactory(),
			transformprocessor.NewFactory(),
			memorylimiterprocessor.NewFactory(),
			resourcedetectionprocessor.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Exporters, err = exporter.MakeFactoryMap(
		[]exporter.Factory{
			NewFactory(),
			loggingexporter.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	factories.Extensions, err = extension.MakeFactoryMap(
		[]extension.Factory{
			healthcheckextension.NewFactory(),
			ballastextension.NewFactory(),
			filestorage.NewFactory(),
			pprofextension.NewFactory(),
			zpagesextension.NewFactory(),
		}...,
	)
	require.NoError(t, err)
	return factories
}
