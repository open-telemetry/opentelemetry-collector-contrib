// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"

// Pools represents the top level json returned by the pool/stats endpoint
type Pools struct {
	Entries map[string]PoolStats `json:"entries"`
}

// PoolStats represents the statistics returned for a single pool
type PoolStats struct {
	NestedStats struct {
		Entries struct {
			Name struct {
				Description string `json:"description,omitempty"`
			} `json:"tmName,omitempty"`
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
			TotalMemberCount struct {
				Value int64 `json:"value"`
			} `json:"memberCnt,omitempty"`
			ActiveMemberCount struct {
				Value int64 `json:"value"`
			} `json:"activeMemberCnt,omitempty"`
			AvailableMemberCount struct {
				Value int64 `json:"value"`
			} `json:"availableMemberCnt,omitempty"`
		} `json:"entries,omitempty"`
	} `json:"nestedStats,omitempty"`
}
