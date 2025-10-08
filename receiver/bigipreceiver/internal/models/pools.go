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
			} `json:"tmName"`
			ServersideBitsIn struct {
				Value int64 `json:"value"`
			} `json:"serverside.bitsIn"`
			ServersideBitsOut struct {
				Value int64 `json:"value"`
			} `json:"serverside.bitsOut"`
			ServersideCurConns struct {
				Value int64 `json:"value"`
			} `json:"serverside.curConns"`
			ServersidePktsIn struct {
				Value int64 `json:"value"`
			} `json:"serverside.pktsIn"`
			ServersidePktsOut struct {
				Value int64 `json:"value"`
			} `json:"serverside.pktsOut"`
			AvailabilityState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.availabilityState"`
			EnabledState struct {
				Description string `json:"description,omitempty"`
			} `json:"status.enabledState"`
			TotalRequests struct {
				Value int64 `json:"value"`
			} `json:"totRequests"`
			TotalMemberCount struct {
				Value int64 `json:"value"`
			} `json:"memberCnt"`
			ActiveMemberCount struct {
				Value int64 `json:"value"`
			} `json:"activeMemberCnt"`
			AvailableMemberCount struct {
				Value int64 `json:"value"`
			} `json:"availableMemberCnt"`
		} `json:"entries"`
	} `json:"nestedStats"`
}
