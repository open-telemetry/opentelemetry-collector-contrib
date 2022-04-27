package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxhreceiver/internal/model"

import "github.com/vmware/go-vmware-nsxt/manager" // NodeNetworkInterfacePropertiesListResult wraps the results of a node's network interface
type NodeNetworkInterfacePropertiesListResult struct {
	// Node network interface property results
	Results []NetworkInterface `json:"results"`
}

// NetworkInterface is one of a node's nics
type NetworkInterface struct {
	*manager.NodeNetworkInterfaceProperties `mapstructure:",squash"`
	ID                                      string `json:"id"`
}
