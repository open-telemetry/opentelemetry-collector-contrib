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

package translate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/translate"
)

func TestEncodingTraceData(t *testing.T) {
	t.Parallel()

	assert.NoError(t, translate.JaegerExporter(nil).WriteTraces(pdata.NewTraces()), "Must not error when processing spans")
}

func TestEncodingMetricData(t *testing.T) {
	t.Parallel()

	assert.Error(t, translate.JaegerExporter(nil).WriteMetrics(pdata.NewMetrics()), "Must error when trying to encode unsupported type")
}

func TestEncodingLogData(t *testing.T) {
	t.Parallel()

	assert.Error(t, translate.JaegerExporter(nil).WriteLogs(pdata.NewLogs()), "Must error when trying to encode unsupported type")
}
