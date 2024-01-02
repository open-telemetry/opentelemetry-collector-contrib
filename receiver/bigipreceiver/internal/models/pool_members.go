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
			} `json:"nodeName,omitempty"`
			PoolName struct {
				Description string `json:"description,omitempty"`
			} `json:"poolName,omitempty"`
			IPAddress struct {
				Description string `json:"description,omitempty"`
			} `json:"addr,omitempty"`
			Port struct {
				Value int64 `json:"value,omitempty"`
			} `json:"port,omitempty"`
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
