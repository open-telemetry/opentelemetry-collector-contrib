// Copyright  The OpenTelemetry Authors
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

package datadogexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func Test_logs_exporter_send_logs(t *testing.T) {

	server := testutils.DatadogServerMock()
	defer server.Close()

	params := componenttest.NewNopExporterCreateSettings()
	f := NewFactory()
	ctx := context.Background()
	cfg := &Config{
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
		},
		Logs: LogsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
		},
	}

	exp, err := f.CreateLogsExporter(ctx, params, cfg)
	require.NoError(t, err)

	ld := testdata.GenerateLogsOneLogRecord()
	require.NoError(t, exp.ConsumeLogs(ctx, ld))

	assert.Equal(t, 1, len(server.LogsData))

	lr := server.LogsData[0]
	assert.Nil(t, lr["ddtags"])
	assert.NotEmpty(t, lr["message"])
	assert.NotEmpty(t, lr["status"])

	// attributes from log need to be added
	assert.NotEmpty(t, lr["app"])

}
