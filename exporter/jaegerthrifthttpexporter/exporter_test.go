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
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
)

const testHTTPAddress = "http://a.test.dom:123/at/some/path"

type args struct {
	config      configmodels.Exporter
	httpAddress string
	headers     map[string]string
	timeout     time.Duration
}

func TestNew(t *testing.T) {
	ar := args{
		config:      &configmodels.ExporterSettings{},
		httpAddress: testHTTPAddress,
		headers:     map[string]string{"test": "test"},
		timeout:     10 * time.Nanosecond,
	}

	got, err := newTraceExporter(ar.config, ar.httpAddress, ar.headers, ar.timeout)
	assert.NoError(t, err)
	require.NotNil(t, got)

	err = got.ConsumeTraces(context.Background(), pdata.NewTraces())
	assert.NoError(t, err)
}

func TestNewFailsWithEmptyExporterName(t *testing.T) {
	ar := args{
		config:      nil,
		httpAddress: testHTTPAddress,
	}

	got, err := newTraceExporter(ar.config, ar.httpAddress, ar.headers, ar.timeout)
	assert.EqualError(t, err, "nil config")
	assert.Nil(t, got)
}
