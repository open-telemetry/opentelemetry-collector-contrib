// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonfileobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/jsonfileobserver"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// EndpointConfig represents a single endpoint configuration from the JSON file.
type EndpointConfig struct {
	// ID is the unique identifier for this endpoint.
	ID string `json:"id"`
	// Target is the endpoint target (e.g., hostname:port).
	Target string `json:"target"`
	// Name is a human-readable name for this endpoint.
	Name string `json:"name"`
	// Labels are arbitrary key-value pairs for filtering.
	Labels map[string]string `json:"labels"`
}

// JSONFileEndpoint represents an endpoint discovered from a JSON file.
type JSONFileEndpoint struct {
	// ID is the unique identifier for this endpoint.
	ID string
	// Name is a human-readable name for this endpoint.
	Name string
	// Labels are arbitrary key-value pairs for filtering.
	Labels map[string]string
}

func (j *JSONFileEndpoint) Env() observer.EndpointEnv {
	return map[string]any{
		"id":     j.ID,
		"name":   j.Name,
		"labels": j.Labels,
	}
}

func (*JSONFileEndpoint) Type() observer.EndpointType {
	return observer.JSONFileType
}

// convertEndpointConfig converts an EndpointConfig to an observer.Endpoint.
func convertEndpointConfig(idNamespace string, ec EndpointConfig) observer.Endpoint {
	id := ec.ID
	if id == "" {
		id = ec.Target
	}

	labels := ec.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	return observer.Endpoint{
		ID:     observer.EndpointID(idNamespace + "/" + id),
		Target: ec.Target,
		Details: &JSONFileEndpoint{
			ID:     id,
			Name:   ec.Name,
			Labels: labels,
		},
	}
}
