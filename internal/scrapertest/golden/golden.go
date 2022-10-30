// Copyright The OpenTelemetry Authors
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
	"os"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// ReadMetrics reads a pmetric.Metrics from the specified file
func ReadMetrics(filePath string) (pmetric.Metrics, error) {
	expectedFileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	unmarshaller := &pmetric.JSONUnmarshaler{}
	return unmarshaller.UnmarshalMetrics(expectedFileBytes)
}

// WriteMetrics writes a pmetric.Metrics to the specified file
func WriteMetrics(filePath string, metrics pmetric.Metrics) error {
	unmarshaler := &pmetric.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalMetrics(metrics)
	if err != nil {
		return err
	}
	var jsonVal map[string]interface{}
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return err
	}
	b, err := json.MarshalIndent(jsonVal, "", "   ")
	if err != nil {
		return err
	}
	b = append(b, []byte("\n")...)
	return os.WriteFile(filePath, b, 0600)
}
