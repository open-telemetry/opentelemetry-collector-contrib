// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereloadereceiver

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
)

func TestRunnerConfig_Validate(t *testing.T) {
	tests := []struct {
		cfg     string
		failed  bool
		message string
		cType   config.DataType
	}{
		{
			message: "test a valid config",
			failed:  false,
			cType:   config.MetricsDataType,
			cfg: `
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'demo'
          scrape_interval: 5s

processors:
  batch:

partial_pipelines:
  metrics:
    receivers: [prometheus]
    processors: [batch]`,
		},
		{
			message: "test an invalid config which has unused receiver",
			failed:  true,
			cType:   config.MetricsDataType,
			cfg: `
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'demo'
          scrape_interval: 5s
  prometheus/1:
processors:
  batch:

partial_pipelines:
  metrics:
    receivers: [prometheus]
    processors: [batch]`,
		},
		{
			message: "test an invalid config which has non existent receiver",
			failed:  true,
			cType:   config.MetricsDataType,
			cfg: `
receivers:
  prom:
processors:
  batch:

partial_pipelines:
  metrics:
    receivers: [prom]
    processors: [batch]`,
		},
		{
			message: "test an invalid config which has non existent processor",
			failed:  true,
			cType:   config.MetricsDataType,
			cfg: `
receivers:
  prometheus:
processors:
  b1:

partial_pipelines:
  metrics:
    receivers: [prometheus]
    processors: [b1]`,
		},
		{
			message: "test environment variable substitution",
			failed:  false,
			cType:   config.MetricsDataType,
			cfg: `
receivers:
  prometheus:
processors:
  batch:

partial_pipelines:
  metrics:
    receivers: [$TESTENV]
    processors: [batch]
`,
		},
		{
			message: "test log receiver with metrics pipeline",
			failed:  true,
			cType:   config.MetricsDataType,
			cfg: `
receivers:
  filelog:
processors:
  batch:

partial_pipelines:
  logs:
    receivers: [filelog]
    processors: [batch]
`,
		},
		{
			message: "test log receiver",
			failed:  false,
			cType:   config.LogsDataType,
			cfg: `
receivers:
  filelog:
processors:
  batch:

partial_pipelines:
  logs:
    receivers: [filelog]
    processors: [batch]
`,
		},
	}

	td, err := os.MkdirTemp("", "")
	require.Nil(t, err)
	defer os.RemoveAll(td)

	os.Setenv("TESTENV", "prometheus")

	for i, test := range tests {
		t.Run(test.message, func(t *testing.T) {
			path := td + "/" + strconv.Itoa(i) + ".yml"
			createFile(t, path, test.cfg)
			_, err := newRunnerConfigFromFile(context.TODO(), path, test.cType)
			if test.failed {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
