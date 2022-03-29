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

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"

import (
	"encoding/json"
	"io/ioutil"

	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
)

// ReadMetrics reads a pdata.Metrics from the specified file
func ReadMetrics(filePath string) (pdata.Metrics, error) {
	expectedFileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return pdata.Metrics{}, err
	}
	unmarshaller := otlp.NewJSONMetricsUnmarshaler()
	return unmarshaller.UnmarshalMetrics(expectedFileBytes)
}

// WriteMetrics writes a pdata.Metrics to the specified file
func WriteMetrics(filePath string, metrics pdata.Metrics) error {
	bytes, err := otlp.NewJSONMetricsMarshaler().MarshalMetrics(metrics)
	if err != nil {
		return err
	}
	var jsonVal map[string]interface{}
	json.Unmarshal(bytes, &jsonVal)
	b, err := json.MarshalIndent(jsonVal, "", "   ")
	if err != nil {
		return err
	}
	b = append(b, []byte("\n")...)
	return ioutil.WriteFile(filePath, b, 0600)
}
