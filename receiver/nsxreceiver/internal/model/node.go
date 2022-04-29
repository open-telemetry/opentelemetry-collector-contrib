package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxhreceiver/internal/model"

type TransportNodeList struct {
	Results []TransportNode `json:"results"`
}

type TransportNode struct {
	ID           string `json:"id"`
	Name         string `json:"display_name"`
	Description  string `json:"description"`
	ResourceType string `json:"resource_type"`

	ManagerRole    *struct{} `json:"manager_role,omitempty"`
	ControllerRole *struct{} `json:"controller_role,omitempty"`
}

type TransportZoneEndpoint struct {
	ZoneID string `json:"transport_zone_id"`
}

// NodeStatListResult Runtime status information of fabric nodes
type NodeStatListResult struct {
	Results []NodeStatus `json:"results"`
}

type TransportNodeStatus struct {
	NodeStatus NodeStatus `json:"node_status"`
}

// NodeStatus is a status for a node
type NodeStatus struct {
	SystemStatus *SystemStatus `json:"system_status"`
}

// SystemStatus is the system status portion of a node's status response
type SystemStatus struct {
	CPUCores        int                 `json:"cpu_cores"`
	DpdkCPUCores    int                 `json:"dpdk_cpu_cores"`
	NonDpdkCPUCores int                 `json:"non_dpdk_cpu_cores"`
	DiskSpaceTotal  int                 `json:"disk_space_total"`
	DiskSpaceUsed   int                 `json:"disk_space_used"`
	FileSystems     []FileSystemsUsage  `json:"file_systems"`
	LoadAverage     []float64           `json:"load_average"`
	CPUUsage        *NodeSystemCPUUsage `json:"cpu_usage"`
	EdgeMemUsage    *MemSystemUsage     `json:"edge_mem_usage"`
	MemCache        int                 `json:"mem_cache"`
	MemTotal        int                 `json:"mem_total"`
	MemUsed         int                 `json:"mem_used"`
	Source          string              `json:"source"`
	SwapTotal       int                 `json:"swap_total"`
	SwapUsed        int                 `json:"swap_used"`
	SystemTime      int64               `json:"system_time"`
	Uptime          int64               `json:"uptime"`
}

type NodeSystemCPUUsage struct {
	HighestCPUCoreUsageDpdk    float64 `json:"highest_cpu_core_usage_dpdk"`
	AvgCPUCoreUsageDpdk        float64 `json:"avg_cpu_core_usage_dpdk"`
	HighestCPUCoreUsageNonDpdk float64 `json:"highest_cpu_core_usage_non_dpdk"`
	AvgCPUCoreUsageNonDpdk     float64 `json:"avg_cpu_core_usage_non_dpdk"`
}

type FileSystemsUsage struct {
	// Name of the filesystem
	FileSystem string `json:"file_system"`
	Mount      string `json:"mount"`
	Total      int    `json:"total"`
	Type       string `json:"type"`
	Used       int    `json:"used"`
}

type MemSystemUsage struct {
	SystemMemUsage     float64 `json:"system_mem_usage"`
	SwapUsage          float64 `json:"swap_usage"`
	CacheUsage         float64 `json:"cache_usage"`
	DatapathTotalUsage float64 `json:"datapath_total_usage"`
}
