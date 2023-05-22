// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gohai // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata/internal/gohai"

// ProcessesPayload handles the JSON unmarshalling
type ProcessesPayload struct {
	Processes map[string]interface{} `json:"processes"`
	Meta      map[string]string      `json:"meta"`
}
