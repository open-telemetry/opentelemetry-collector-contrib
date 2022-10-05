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

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, _ := servicetest.LoadConfig(filepath.Join("testdata", "config.yaml"), factories)

	require.NotNil(t, cfg)

	// There are 3 stanzas in the exporter.
	assert.Equal(t, len(cfg.Exporters), 3)

	exporter := cfg.Exporters[config.NewComponentID(typeStr)]

	// Is a valid configuration that has no errors
	assert.NoError(t, configtest.CheckConfigStruct(exporter))
	assert.Equal(
		t,
		&Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "")),
			ClusterURI:       "https://CLUSTER.kusto.windows.net",
			ApplicationID:    "f80da32c-108c-415c-a19e-643f461a677a",
			ApplicationKey:   "xx-xx-xx-xx",
			TenantID:         "21ff9e36-fbaa-43c8-98ba-00431ea10bc3",
			Database:         "oteldb",
			MetricTable:      "OTELMetrics",
			LogTable:         "OTELLogs",
			TraceTable:       "OTELTraces",
			IngestionType:    managedIngestType,
		},
		exporter)

	// The second one has a validation error
	exporter = cfg.Exporters[config.NewComponentIDWithName(typeStr, "2")].(*Config)
	require.NotNil(t, exporter)
	err2 := exporter.Validate()
	require.NotNil(t, err2)
	assert.EqualError(t, err2, `mandatory configurations "cluster_uri" ,"application_id" , "application_key" and "tenant_id" are missing or empty `)

	// The third one has an error wrto ingest type
	exporter = cfg.Exporters[config.NewComponentIDWithName(typeStr, "3")].(*Config)
	require.NotNil(t, exporter)
	err3 := exporter.Validate()
	require.NotNil(t, err3)
	assert.EqualError(t, err3, `unsupported configuration for ingestion_type. Accepted types [managed, queued] Provided [streaming]`)
}
