// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver/internal/model"

// ClusterNodeList is a result struct from
type ClusterNodeList struct {
	Results []ClusterNode `json:"results"`
}

// ClusterNode is a Controller Node or Manager Node
type ClusterNode struct {
	NodeProperties `mapstructure:",squash"`
	ControllerRole *ControllerRole `json:"controller_role,omitempty"`
}

// ControllerRole is a collection of information specific to controller nodes
type ControllerRole struct {
	Type string `json:"type"`
}

// TransportNodeList is a list of Transport Nodes
type TransportNodeList struct {
	Results []TransportNode `json:"results"`
}

// TransportNode is a representation of an NSX host or edge transport node
type TransportNode struct {
	NodeProperties `mapstructure:",squash"`
	Description    string `json:"description" `
}

// NodeProperties are identifiers of a node in NSX
type NodeProperties struct {
	ID           string `json:"id" mapstructure:"node_id"`
	Name         string `json:"display_name,omitempty" `
	ResourceType string `json:"resource_type"`
}

// NodeStatus is a status for a node
type NodeStatus struct {
	LastHeartbeatTimestamp       int64  `json:"last_heartbeat_timestamp"`
	MpaConnectivityStatus        string `json:"mpa_connectivity_status"`
	MpaConnectivityStatusDetails string `json:"mpa_connectivity_status_details"`
	LcpConnectivityStatus        string `json:"lcp_connectivity_status"`
	LcpConnectivityStatusDetails []struct {
		ControlNodeIP string `json:"control_node_ip"`
		Status        string `json:"status"`
	} `json:"lcp_connectivity_status_details"`
	HostNodeDeploymentStatus string `json:"host_node_deployment_status"`
	SoftwareVersion          string `json:"software_version"`
	SystemStatus             struct {
		CPUCores        int `json:"cpu_cores"`
		DpdkCPUCores    int `json:"dpdk_cpu_cores"`
		NonDpdkCPUCores int `json:"non_dpdk_cpu_cores"`
		DiskSpaceTotal  int `json:"disk_space_total"`
		DiskSpaceUsed   int `json:"disk_space_used"`
		FileSystems     []struct {
			FileSystem string `json:"file_system"`
			Mount      string `json:"mount"`
			Total      int    `json:"total"`
			Type       string `json:"type"`
			Used       int    `json:"used"`
		} `json:"file_systems"`
		LoadAverage []float64 `json:"load_average"`
		CPUUsage    struct {
			HighestCPUCoreUsageDpdk    float64 `json:"highest_cpu_core_usage_dpdk"`
			AvgCPUCoreUsageDpdk        float64 `json:"avg_cpu_core_usage_dpdk"`
			HighestCPUCoreUsageNonDpdk float64 `json:"highest_cpu_core_usage_non_dpdk"`
			AvgCPUCoreUsageNonDpdk     float64 `json:"avg_cpu_core_usage_non_dpdk"`
		} `json:"cpu_usage"`
		EdgeMemUsage *struct {
			SystemMemUsage          float64 `json:"system_mem_usage"`
			SwapUsage               float64 `json:"swap_usage"`
			CacheUsage              float64 `json:"cache_usage"`
			DatapathTotalUsage      float64 `json:"datapath_total_usage"`
			DatapathMemUsageDetails struct {
				DatapathHeapUsage                float64  `json:"datapath_heap_usage"`
				HighestDatapathMemPoolUsage      float64  `json:"highest_datapath_mem_pool_usage"`
				HighestDatapathMemPoolUsageNames []string `json:"highest_datapath_mem_pool_usage_names"`
				DatapathMemPoolsUsage            []struct {
					Name        string  `json:"name"`
					Description string  `json:"description"`
					Usage       float64 `json:"usage"`
				} `json:"datapath_mem_pools_usage"`
			} `json:"datapath_mem_usage_details"`
		} `json:"edge_mem_usage,omitempty"`
		MemCache   int    `json:"mem_cache"`
		MemTotal   int    `json:"mem_total"`
		MemUsed    int    `json:"mem_used"`
		Source     string `json:"source"`
		SwapTotal  int    `json:"swap_total"`
		SwapUsed   int    `json:"swap_used"`
		SystemTime int64  `json:"system_time"`
		Uptime     int64  `json:"uptime"`
	} `json:"system_status"`
}

// TransportNodeStatus wraps a node_status because it is wrapped in the HTTP response
type TransportNodeStatus struct {
	NodeStatus NodeStatus `mapstructure:"node_status" json:"node_status"`
}
