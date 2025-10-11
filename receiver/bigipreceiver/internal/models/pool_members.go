// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"

// PoolMembers represents the top level json returned by the members/stats endpoint
type PoolMembers struct {
	Entries map[string]PoolMemberStats `json:"entries"`
}

// PoolMemberStats represents the statistics returned for a single pool member
type PoolMemberStats struct {
	NestedStats struct {
		Entries struct {
			Name struct {
				Description string `json:"description,omitempty"`
			} `json:"nodeName,omitzero"`
			PoolName struct {
				Description string `json:"description,omitempty"`
			} `json:"poolName,omitzero"`
			IPAddress struct {
				Description string `json:"description,omitempty"`
			} `json:"addr,omitzero"`
			Port struct {
				Value int64 `json:"value,omitempty"`
			} `json:"port,omitzero"`
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
