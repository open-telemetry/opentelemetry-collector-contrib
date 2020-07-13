package stats

// NodeInfoOutput holds output from "_nodes/_local" endpoint
type NodeInfoOutput struct {
	ClusterName *string             `json:"cluster_name"`
	Nodes       map[string]NodeInfo `json:"nodes"`
}

// NodeInfo contains basic information from the nodes
type NodeInfo struct {
	Name             *string `json:"name"`
	TransportAddress *string `json:"transport_address"`
	Host             *string `json:"host"`
	IP               *string `json:"ip"`
	Version          *string `json:"version"`
}

// MasterInfoOutput holds output from "_cluster/state/master_node"
type MasterInfoOutput struct {
	ClusterName *string `json:"cluster_name"`
	MasterNode  *string `json:"master_node"`
}
