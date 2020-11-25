// Copyright 2020, OpenTelemetry Authors
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

package sumologicexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestInitExporter(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
	})
	assert.NoError(t, err)
}

func TestInitExporterInvalidLogFormat(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "test_format",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
	})

	assert.EqualError(t, err, "unexpected log format: test_format")
}

func TestInitExporterInvalidMetricFormat(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:    "json",
		MetricFormat: "test_format",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
		CompressEncoding: "gzip",
	})

	assert.EqualError(t, err, "unexpected metric format: test_format")
}

func TestInitExporterInvalidCompressEncoding(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "test_format",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:  defaultTimeout,
			Endpoint: "test_endpoint",
		},
	})

	assert.EqualError(t, err, "unexpected compression encoding: test_format")
}

func TestInitExporterInvalidEndpoint(t *testing.T) {
	_, err := initExporter(&Config{
		LogFormat:        "json",
		MetricFormat:     "carbon2",
		CompressEncoding: "gzip",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: defaultTimeout,
		},
	})

	assert.EqualError(t, err, "endpoint is not set")
}
