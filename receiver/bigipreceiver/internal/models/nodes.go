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
			} `json:"tmName,omitempty"`
			IPAddress struct {
				Description string `json:"description,omitempty"`
			} `json:"addr,omitempty"`
			ServersideBitsIn struct {
				Value int64 `json:"value"`
			} `json:"serverside.bitsIn,omitempty"`
			ServersideBitsOut struct {
				Value int64 `json:"value"`
			} `json:"serverside.bitsOut,omitempty"`
			ServersideCurConns struct {
				Value int64 `json:"value"`
			} `json:"serverside.curConns,omitempty"`
			ServersidePktsIn struct {
				Value int64 `json:"value"`
			} `json:"serverside.pktsIn,omitempty"`
			ServersidePktsOut struct {
				Value int64 `json:"value"`
			} `json:"serverside.pktsOut,omitempty"`
			AvailabilityState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.availabilityState,omitempty"`
			EnabledState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.enabledState,omitempty"`
			TotalRequests struct {
				Value int64 `json:"value"`
			} `json:"totRequests,omitempty"`
			CurSessions struct {
				Value int64 `json:"value"`
			} `json:"curSessions,omitempty"`
		} `json:"entries,omitempty"`
	} `json:"nestedStats,omitempty"`
}
