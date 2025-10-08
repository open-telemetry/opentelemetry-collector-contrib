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
			} `json:"nodeName"`
			PoolName struct {
				Description string `json:"description,omitempty"`
			} `json:"poolName"`
			IPAddress struct {
				Description string `json:"description,omitempty"`
			} `json:"addr"`
			Port struct {
				Value int64 `json:"value,omitempty"`
			} `json:"port"`
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
			CurSessions struct {
				Value int64 `json:"value"`
			} `json:"curSessions"`
		} `json:"entries"`
	} `json:"nestedStats"`
}
