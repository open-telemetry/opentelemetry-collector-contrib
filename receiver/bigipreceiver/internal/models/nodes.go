// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"

// Nodes represents the top level json returned by the node/stats endpoint
type Nodes struct {
	Entries map[string]NodeStats `json:"entries"`
}

// NodeStats represents the statistics returned for a single node
type NodeStats struct {
	NestedStats struct {
		Entries struct {
			Name struct {
				Description string `json:"description,omitempty"`
			} `json:"tmName,omitzero"`
			IPAddress struct {
				Description string `json:"description,omitempty"`
			} `json:"addr,omitzero"`
			ServersideBitsIn struct {
				Value int64 `json:"value"`
			} `json:"serverside.bitsIn,omitzero"`
			ServersideBitsOut struct {
				Value int64 `json:"value"`
			} `json:"serverside.bitsOut,omitzero"`
			ServersideCurConns struct {
				Value int64 `json:"value"`
			} `json:"serverside.curConns,omitzero"`
			ServersidePktsIn struct {
				Value int64 `json:"value"`
			} `json:"serverside.pktsIn,omitzero"`
			ServersidePktsOut struct {
				Value int64 `json:"value"`
			} `json:"serverside.pktsOut,omitzero"`
			AvailabilityState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.availabilityState,omitzero"`
			EnabledState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.enabledState,omitzero"`
			TotalRequests struct {
				Value int64 `json:"value"`
			} `json:"totRequests,omitzero"`
			CurSessions struct {
				Value int64 `json:"value"`
			} `json:"curSessions,omitzero"`
		} `json:"entries,omitzero"`
	} `json:"nestedStats,omitzero"`
}
