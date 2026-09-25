// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/model"

// MasterNodeResponse represents a response from elasticsearch's /_cluster/state/master_node endpoint.
type MasterNodeResponse struct {
	MasterNode string `json:"master_node"`
}
