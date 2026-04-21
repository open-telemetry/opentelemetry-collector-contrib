// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"

// IndexStats represents a response from elasticsearch's /_stats endpoint.
// The struct is not exhaustive; It does not provide all values returned by elasticsearch,
// only the ones relevant to the metrics retrieved by the scraper.
type IndexStats struct {
	All     IndexStatsIndexInfo             `json:"_all"`
	Indices map[string]*IndexStatsIndexInfo `json:"indices"`
}

type IndexStatsIndexInfo struct {
	Primaries NodeStatsNodesInfoIndices `json:"primaries"`
	Total     NodeStatsNodesInfoIndices `json:"total"`
}
