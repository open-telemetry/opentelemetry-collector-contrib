// Copyright 2019, OpenTelemetry Authors
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

package jaegerthrifthttpexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/model/pdata"
)

const testHTTPAddress = "http://a.example.com:123/at/some/path"

func TestNew(t *testing.T) {
	config := Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: testHTTPAddress,
			Headers:  map[string]string{"test": "test"},
			Timeout:  10 * time.Nanosecond,
		},
	}

	got, err := newTracesExporter(&config, componenttest.NewNopExporterCreateSettings())
	assert.NoError(t, err)
	require.NotNil(t, got)

	err = got.ConsumeTraces(context.Background(), pdata.NewTraces())
	assert.NoError(t, err)
}
