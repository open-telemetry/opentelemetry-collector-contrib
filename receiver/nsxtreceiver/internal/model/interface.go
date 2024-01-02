// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver/internal/model"

import "github.com/vmware/go-vmware-nsxt/manager" // NodeNetworkInterfacePropertiesListResult wraps the results of a node's network interface

// NodeNetworkInterfacePropertiesListResult contains the results of the Node's Network Interfaces
type NodeNetworkInterfacePropertiesListResult struct {
	// Node network interface property results
	Results []NetworkInterface `json:"results"`
}

// NetworkInterface is one of a node's nics
type NetworkInterface struct {
	*manager.NodeNetworkInterfaceProperties `mapstructure:",squash"`
}

// NetworkInterfaceStats are the statistics on a node's network interface
type NetworkInterfaceStats struct {
	*manager.NodeInterfaceStatisticsProperties `mapstructure:",squash"`
}
