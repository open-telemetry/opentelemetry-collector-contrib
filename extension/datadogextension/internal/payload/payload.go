// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// package payload will define the metadata payload schemas to be forwarded to Datadog backend
package payload // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"

import (
	"encoding/json"
	"errors"

	"github.com/DataDog/datadog-agent/pkg/serializer/marshaler"
)

const (
	payloadSplitErr = "could not split otel collector payload any more, payload is too big for intake"
)

// CustomBuildInfo is a struct that duplicates the fields of component.BuildInfo with custom JSON tags
type CustomBuildInfo struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type OtelCollector struct {
	HostKey           string             `json:"host_key"`
	Hostname          string             `json:"hostname"`
	HostnameSource    string             `json:"hostname_source"`
	CollectorID       string             `json:"collector_id"`
	CollectorVersion  string             `json:"collector_version"`
	ConfigSite        string             `json:"config_site"`
	APIKeyUUID        string             `json:"api_key_uuid"`
	FullComponents    []CollectorModule  `json:"full_components"`
	ActiveComponents  []ServiceComponent `json:"active_components"`
	BuildInfo         CustomBuildInfo    `json:"build_info"`
	FullConfiguration string             `json:"full_configuration"` // JSON passed as string
	HealthStatus      string             `json:"health_status"`      // JSON passed as string
}

type CollectorModule struct {
	Type       string `json:"type"`
	Kind       string `json:"kind"`
	Gomod      string `json:"gomod"`
	Version    string `json:"version"`
	Configured bool   `json:"configured"`
}

type ServiceComponent struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Kind            string `json:"kind"`
	Pipeline        string `json:"pipeline"`
	Gomod           string `json:"gomod"`
	Version         string `json:"version"`
	ComponentStatus string `json:"component_status"`
}

// Explicitly implement JSONMarshaler interface from datadog-agent
var (
	_ marshaler.JSONMarshaler = (*OtelCollectorPayload)(nil)
)

type OtelCollectorPayload struct {
	Hostname  string        `json:"hostname"`
	Timestamp int64         `json:"timestamp"`
	Metadata  OtelCollector `json:"otel_collector"`
	UUID      string        `json:"uuid"`
}

// MarshalJSON serializes a OtelCollectorPayload to JSON
func (p *OtelCollectorPayload) MarshalJSON() ([]byte, error) {
	type collectorPayloadAlias OtelCollectorPayload
	return json.Marshal((*collectorPayloadAlias)(p))
}

// SplitPayload implements marshaler.AbstractMarshaler#SplitPayload.
func (p *OtelCollectorPayload) SplitPayload(_ int) ([]marshaler.AbstractMarshaler, error) {
	return nil, errors.New(payloadSplitErr)
}

// PrepareOtelCollectorPayload takes metadata from various config values and prepares an OtelCollector payload
func PrepareOtelCollectorPayload(
	hostname,
	hostnameSource,
	extensionUUID,
	version,
	site,
	fullConfig string,
	buildInfo CustomBuildInfo,
) OtelCollector {
	return OtelCollector{
		HostKey:           "",
		Hostname:          hostname,
		HostnameSource:    hostnameSource,
		CollectorID:       hostname + "-" + extensionUUID,
		CollectorVersion:  version,
		ConfigSite:        site,
		APIKeyUUID:        "",
		BuildInfo:         buildInfo,
		FullConfiguration: fullConfig,
	}
}
