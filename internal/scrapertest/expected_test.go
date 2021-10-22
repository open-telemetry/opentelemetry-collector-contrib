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

package scrapertest

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestMetricsToFile(t *testing.T) {
	metricslice := baseTestMetrics()
	metrics := pdata.NewMetrics()
	metricslice.CopyTo(metrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics())

	tempDir := filepath.Join(t.TempDir(), "metrics.json")
	WriteExpected(tempDir, metrics)

	actualBytes, err := ioutil.ReadFile(tempDir)
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "roundtrip", "expected.json")
	expectedBytes, err := ioutil.ReadFile(expectedFile)
	require.NoError(t, err)

	require.Equal(t, expectedBytes, actualBytes)
}

func TestFileToMetrics(t *testing.T) {
	metricslice := baseTestMetrics()
	expectedMetrics := pdata.NewMetrics()
	metricslice.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics())

	expectedFile := filepath.Join("testdata", "roundtrip", "expected.json")
	actualMetrics, err := ReadExpected(expectedFile)
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, actualMetrics)
}
