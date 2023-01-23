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

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
func WriteMetrics(t *testing.T, filePath string, metrics pmetric.Metrics) error {
	if err := writeMetrics(filePath, metrics); err != nil {
		return err
	}
	t.Logf("Golden file successfully written to %s.", filePath)
	t.Log("NOTE: The WriteMetrics call must be removed in order to pass the test.")
	t.Fail()
	return nil
}

// writeMetrics writes a pmetric.Metrics to the specified file
func writeMetrics(filePath string, metrics pmetric.Metrics) error {
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
	if err := os.WriteFile(filePath, b, 0600); err != nil {
		return err
	}
	return nil
}

// ReadLogs reads a plog.Logs from the specified file
func ReadLogs(filePath string) (plog.Logs, error) {
	b, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return plog.Logs{}, err
	}

	unmarshaler := plog.JSONUnmarshaler{}
	return unmarshaler.UnmarshalLogs(b)
}

// WriteLogs writes a plog.Logs to the specified file
func WriteLogs(t *testing.T, filePath string, metrics plog.Logs) error {
	if err := writeLogs(filePath, metrics); err != nil {
		return err
	}
	t.Logf("Golden file successfully written to %s.", filePath)
	t.Log("NOTE: The WriteLogs call must be removed in order to pass the test.")
	t.Fail()
	return nil
}

// writeLogs writes a plog.Logs to the specified file
func writeLogs(filePath string, logs plog.Logs) error {
	unmarshaler := &plog.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}
	var jsonVal map[string]interface{}
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return err
	}
	b, err := json.MarshalIndent(jsonVal, "", "    ")
	if err != nil {
		return err
	}
	b = append(b, []byte("\n")...)
	return os.WriteFile(filePath, b, 0600)
}

// ReadTraces reads a ptrace.Traces from the specified file
func ReadTraces(filePath string) (ptrace.Traces, error) {
	b, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return ptrace.Traces{}, err
	}

	unmarshaler := ptrace.JSONUnmarshaler{}
	return unmarshaler.UnmarshalTraces(b)
}

func WriteTraces(t *testing.T, filePath string, traces ptrace.Traces) error {
	if err := writeTraces(filePath, traces); err != nil {
		return err
	}
	t.Logf("Golden file successfully written to %s.", filePath)
	t.Log("NOTE: The WriteTraces call must be removed in order to pass the test.")
	t.Fail()
	return nil
}

// writeTraces writes a ptrace.Traces to the specified file
func writeTraces(filePath string, traces ptrace.Traces) error {
	unmarshaler := &ptrace.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}
	var jsonVal map[string]interface{}
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return err
	}
	b, err := json.MarshalIndent(jsonVal, "", "    ")
	if err != nil {
		return err
	}
	b = append(b, []byte("\n")...)
	return os.WriteFile(filePath, b, 0600)
}
