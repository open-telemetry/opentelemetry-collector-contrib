// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v3"
)

// ReadMetrics reads a pmetric.Metrics from the specified YAML or JSON file.
func ReadMetrics(filePath string) (pmetric.Metrics, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		var m map[string]any
		if err = yaml.Unmarshal(b, &m); err != nil {
			return pmetric.Metrics{}, err
		}
		b, err = json.Marshal(m)
		if err != nil {
			return pmetric.Metrics{}, err
		}
	}
	unmarshaller := &pmetric.JSONUnmarshaler{}
	return unmarshaller.UnmarshalMetrics(b)
}

// WriteMetrics writes a pmetric.Metrics to the specified file in YAML format.
func WriteMetrics(tb testing.TB, filePath string, metrics pmetric.Metrics, opts ...WriteMetricsOption) error {
	if err := writeMetrics(filePath, metrics, opts...); err != nil {
		return err
	}
	tb.Logf("Golden file successfully written to %s.", filePath)
	tb.Log("NOTE: The WriteMetrics call must be removed in order to pass the test.")
	tb.Fail()
	return nil
}

// MarshalMetricsYAML marshals a pmetric.Metrics to YAML format.
func MarshalMetricsYAML(metrics pmetric.Metrics) ([]byte, error) {
	unmarshaler := &pmetric.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalMetrics(metrics)
	if err != nil {
		return nil, err
	}
	var jsonVal map[string]any
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return nil, err
	}
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	if err := enc.Encode(jsonVal); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// writeMetrics writes a pmetric.Metrics to the specified file in YAML format.
func writeMetrics(filePath string, metrics pmetric.Metrics, opts ...WriteMetricsOption) error {
	optsStruct := writeMetricsOptions{
		normalizeTimestamps: true,
	}

	for _, opt := range opts {
		opt(&optsStruct)
	}

	sortMetrics(metrics)
	if optsStruct.normalizeTimestamps {
		normalizeTimestamps(metrics)
	}

	b, err := MarshalMetricsYAML(metrics)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, b, 0o600)
}

// ReadLogs reads a plog.Logs from the specified YAML or JSON file.
func ReadLogs(filePath string) (plog.Logs, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return plog.Logs{}, err
	}
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		var m map[string]any
		if err = yaml.Unmarshal(b, &m); err != nil {
			return plog.Logs{}, err
		}
		b, err = json.Marshal(m)
		if err != nil {
			return plog.Logs{}, err
		}
	}
	unmarshaler := plog.JSONUnmarshaler{}
	return unmarshaler.UnmarshalLogs(b)
}

// WriteLogs writes a plog.Logs to the specified file in YAML format.
func WriteLogs(tb testing.TB, filePath string, logs plog.Logs) error {
	if err := writeLogs(filePath, logs); err != nil {
		return err
	}
	tb.Logf("Golden file successfully written to %s.", filePath)
	tb.Log("NOTE: The WriteLogs call must be removed in order to pass the test.")
	tb.Fail()
	return nil
}

// writeLogs writes a plog.Logs to the specified file in YAML format.
func writeLogs(filePath string, logs plog.Logs) error {
	unmarshaler := &plog.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}
	var jsonVal map[string]any
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return err
	}
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	if err := enc.Encode(jsonVal); err != nil {
		return err
	}
	return os.WriteFile(filePath, b.Bytes(), 0o600)
}

// ReadTraces reads a ptrace.Traces from the specified YAML or JSON file.
func ReadTraces(filePath string) (ptrace.Traces, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return ptrace.Traces{}, err
	}
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		var m map[string]any
		if err = yaml.Unmarshal(b, &m); err != nil {
			return ptrace.Traces{}, err
		}
		b, err = json.Marshal(m)
		if err != nil {
			return ptrace.Traces{}, err
		}
	}
	unmarshaler := ptrace.JSONUnmarshaler{}
	return unmarshaler.UnmarshalTraces(b)
}

// WriteTraces writes a ptrace.Traces to the specified file in YAML format.
func WriteTraces(tb testing.TB, filePath string, traces ptrace.Traces) error {
	if err := writeTraces(filePath, traces); err != nil {
		return err
	}
	tb.Logf("Golden file successfully written to %s.", filePath)
	tb.Log("NOTE: The WriteTraces call must be removed in order to pass the test.")
	tb.Fail()
	return nil
}

// writeTraces writes a ptrace.Traces to the specified file
func writeTraces(filePath string, traces ptrace.Traces) error {
	unmarshaler := &ptrace.JSONMarshaler{}
	fileBytes, err := unmarshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}
	var jsonVal map[string]any
	if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
		return err
	}
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	if err := enc.Encode(jsonVal); err != nil {
		return err
	}
	return os.WriteFile(filePath, b.Bytes(), 0o600)
}
