// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package observiqexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtest"
)

const exampleAPIKey = "11111111-2222-3333-4444-555555555555"

func TestNewFactory(t *testing.T) {
	fact := NewFactory()
	require.NotNil(t, fact, "failed to create new factory")
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.APIKey = exampleAPIKey
	_, err := createLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	require.NoError(t, err)
}

func TestCreateLogsExporterNilConfig(t *testing.T) {
	_, err := createLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), nil)
	require.Error(t, err)
}
