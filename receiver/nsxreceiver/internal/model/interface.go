// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxhreceiver/internal/model"

import "github.com/vmware/go-vmware-nsxt/manager" // NodeNetworkInterfacePropertiesListResult wraps the results of a node's network interface
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
