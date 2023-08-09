// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"

// Nodes represents a response from elasticsearch's /_nodes.
// The struct is not exhaustive; It does not provide all values returned by elasticsearch,
// only the ones relevant to the metrics retrieved by the scraper.
type Nodes struct {
	ClusterName string              `json:"cluster_name"`
	Nodes       map[string]NodeInfo `json:"nodes"`
}

type NodeInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
