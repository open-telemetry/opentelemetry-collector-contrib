// Copyright OpenTelemetry Authors
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

package azureblobreceiver

// func TestCreateTracesExporterUsingDefaultConfig(t *testing.T) {
// 	ctx := context.Background()
// 	params := componenttest.NewNopExporterCreateSettings()
// 	exporter, err := createTracesExporter(ctx, params, createDefaultConfig())

// 	require.Nil(t, err)
// 	assert.NotNil(t, exporter)
// }

// func TestCreateLogsExporterUsingDefaultConfig(t *testing.T) {
// 	ctx := context.Background()
// 	params := componenttest.NewNopExporterCreateSettings()
// 	exporter, err := createLogsExporter(ctx, params, createDefaultConfig())

// 	require.Nil(t, err)
// 	assert.NotNil(t, exporter)
// }

// func TestCreateTracesExporterUsingSpecificConfig(t *testing.T) {
// 	ctx := context.Background()
// 	params := componenttest.NewNopExporterCreateSettings()
// 	exporter, err := createTracesExporter(ctx, params, createSpecificConfig())

// 	require.Nil(t, err)
// 	assert.NotNil(t, exporter)
// }

// func TestCreateLogsExporterUsingSpecificConfig(t *testing.T) {
// 	ctx := context.Background()
// 	params := componenttest.NewNopExporterCreateSettings()
// 	exporter, err := createLogsExporter(ctx, params, createSpecificConfig())

// 	require.Nil(t, err)
// 	assert.NotNil(t, exporter)
// }

// func createSpecificConfig() config.Exporter {
// 	return &Config{
// 		ExporterSettings:    config.NewExporterSettings(config.NewComponentID(typeStr)),
// 		ConnectionString:    goodConnectionString,
// 		LogsContainerName:   logsContainerName,
// 		TracesContainerName: tracesContainerName,
// 	}
// }
