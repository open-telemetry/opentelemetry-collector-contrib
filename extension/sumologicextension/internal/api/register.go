// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package api // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/api"

type OpenRegisterRequestPayload struct {
	CollectorName string         `json:"collectorName"`
	Ephemeral     bool           `json:"ephemeral,omitempty"`
	Description   string         `json:"description,omitempty"`
	Hostname      string         `json:"hostname,omitempty"`
	Category      string         `json:"category,omitempty"`
	TimeZone      string         `json:"timeZone,omitempty"`
	Clobber       bool           `json:"clobber,omitempty"`
	Fields        map[string]any `json:"fields,omitempty"`
}

type OpenRegisterResponsePayload struct {
	CollectorCredentialID  string `json:"collectorCredentialID"`
	CollectorCredentialKey string `json:"collectorCredentialKey"`
	CollectorID            string `json:"collectorId"`
	CollectorName          string `json:"collectorName"`
}
