package model

import (
	"strconv"
	"strings"
)

const (
	statisticsKey  = "statistics"
	fieldSeparator = ";" // Separates key=value pairs
)

// NodeInfo is the data we want to collect from the INFO response
type NodeInfo struct {
	Name       string     `json:"node"`       // ID of the node
	Services   []string   `json:"services"`   // Endpoints of peer nodes
	Namespaces []string   `json:"namespaces"` // Names of namespaces
	Statistics *NodeStats `json:"statistics"` // Node-level statistics
}

// NodeStats is the data we want to collect from the 'statistics' field of the INFO response
type NodeStats struct {
	ClientConnections          *int64 `json:"client_connections"`
	ClientConnectionsClosed    *int64 `json:"client_connections_closed"`
	ClientConnectionsOpened    *int64 `json:"client_connections_opened"`
	FabricConnections          *int64 `json:"fabric_connections"`
	FabricConnectionsClosed    *int64 `json:"fabric_connections_closed"`
	FabricConnectionsOpened    *int64 `json:"fabric_connections_opened"`
	HeartbeatConnections       *int64 `json:"heartbeat_connections"`
	HeartbeatConnectionsClosed *int64 `json:"heartbeat_connections_closed"`
	HeartbeatConnectionsOpened *int64 `json:"heartbeat_connections_opened"`
	SystemFreeMemPct           *int64 `json:"system_free_mem_pct"`
}

// NamespaceInfo is specific information about a single namespace on a node
type NamespaceInfo struct {
	Name                    string `json:"name"`
	Node                    string `json:"node_name"`
	DeviceAvailablePct      *int64 `json:"device_available_pct"`
	DeviceUsedBytes         *int64 `json:"device_used_bytes"`
	HighWaterDiskPct        *int64 `json:"high-water-disk-pct"`
	HighWaterMemoryPct      *int64 `json:"high-water-memory-pct"`
	MemoryFreePct           *int64 `json:"memory_free_pct"`
	MemoryUsedDataBytes     *int64 `json:"memory_used_data_bytes"`
	MemoryUsedIndexBytes    *int64 `json:"memory_used_index_bytes"`
	MemoryUsedSetIndexBytes *int64 `json:"memory_used_set_index_bytes"`
	MemoryUsedSIndexBytes   *int64 `json:"memory_used_sindex_bytes"`
	ScanAggrAbort           *int64 `json:"scan_aggr_abort"`
	ScanAggrComplete        *int64 `json:"scan_aggr_complete"`
	ScanAggrError           *int64 `json:"scan_aggr_error"`
	ScanBasicAbort          *int64 `json:"scan_basic_abort"`
	ScanBasicComplete       *int64 `json:"scan_basic_complete"`
	ScanBasicError          *int64 `json:"scan_basic_error"`
	ScanOpsBgAbort          *int64 `json:"scan_ops_bg_abort"`
	ScanOpsBgComplete       *int64 `json:"scan_ops_bg_complete"`
	ScanOpsBgError          *int64 `json:"scan_ops_bg_error"`
	ScanUdfBgAbort          *int64 `json:"scan_udf_bg_abort"`
	ScanUdfBgComplete       *int64 `json:"scan_udf_bg_complete"`
	ScanUdfBgError          *int64 `json:"scan_udf_bg_error"`
}

// InfoResponse is the map returned by the INFO command
type InfoResponse map[string]string

// ParseInfo creates NodeInfo using values present in the INFO response
func ParseInfo(i InfoResponse) *NodeInfo {
	nodeInfo := NodeInfo{}

	if v, ok := i["node"]; ok {
		nodeInfo.Name = v
	}
	if v, ok := i["services"]; ok {
		nodeInfo.Services = splitFields(v)
	}
	if v, ok := i["namespaces"]; ok {
		nodeInfo.Namespaces = splitFields(v)
	}
	// If statistics aren't present, keep Statistics a nil pointer
	if _, ok := i[statisticsKey]; ok {
		nodeInfo.Statistics = parseNodeStats(valueToMap(i, statisticsKey))
	}

	return &nodeInfo
}

// ParseNamespaceInfo creates NamespaceInfo using values present in the INFO response
func ParseNamespaceInfo(i InfoResponse, namespace string) *NamespaceInfo {
	infoMap := valueToMap(i, NamespaceKey(namespace))
	namespaceInfo := NamespaceInfo{
		Name: namespace,
	}

	// No point in checking an empty map
	if len(infoMap) == 0 {
		return &namespaceInfo
	}

	if v := parseInt(infoMap, "memory_used_data_bytes"); v != nil {
		namespaceInfo.MemoryUsedDataBytes = v
	}
	if v := parseInt(infoMap, "memory_used_index_bytes"); v != nil {
		namespaceInfo.MemoryUsedIndexBytes = v
	}
	if v := parseInt(infoMap, "memory_used_set_index_bytes"); v != nil {
		namespaceInfo.MemoryUsedSetIndexBytes = v
	}
	if v := parseInt(infoMap, "memory_used_sindex_bytes"); v != nil {
		namespaceInfo.MemoryUsedSIndexBytes = v
	}
	if v := parseInt(infoMap, "memory_free_pct"); v != nil {
		namespaceInfo.MemoryFreePct = v
	}
	if v := parseInt(infoMap, "high-water-memory-pct"); v != nil {
		namespaceInfo.HighWaterMemoryPct = v
	}

	if v := parseInt(infoMap, "device_available_pct"); v != nil {
		namespaceInfo.DeviceAvailablePct = v
	}
	if v := parseInt(infoMap, "high-water-disk-pct"); v != nil {
		namespaceInfo.HighWaterDiskPct = v
	}
	if v := parseInt(infoMap, "device_used_bytes"); v != nil {
		namespaceInfo.DeviceUsedBytes = v
	}

	if v := parseInt(infoMap, "scan_aggr_abort"); v != nil {
		namespaceInfo.ScanAggrAbort = v
	}
	if v := parseInt(infoMap, "scan_aggr_complete"); v != nil {
		namespaceInfo.ScanAggrComplete = v
	}
	if v := parseInt(infoMap, "scan_aggr_error"); v != nil {
		namespaceInfo.ScanAggrError = v
	}
	if v := parseInt(infoMap, "scan_basic_abort"); v != nil {
		namespaceInfo.ScanBasicAbort = v
	}
	if v := parseInt(infoMap, "scan_basic_complete"); v != nil {
		namespaceInfo.ScanBasicComplete = v
	}
	if v := parseInt(infoMap, "scan_basic_error"); v != nil {
		namespaceInfo.ScanBasicError = v
	}
	if v := parseInt(infoMap, "scan_ops_bg_abort"); v != nil {
		namespaceInfo.ScanOpsBgAbort = v
	}
	if v := parseInt(infoMap, "scan_ops_bg_complete"); v != nil {
		namespaceInfo.ScanOpsBgComplete = v
	}
	if v := parseInt(infoMap, "scan_ops_bg_error"); v != nil {
		namespaceInfo.ScanOpsBgError = v
	}
	if v := parseInt(infoMap, "scan_udf_bg_abort"); v != nil {
		namespaceInfo.ScanUdfBgAbort = v
	}
	if v := parseInt(infoMap, "scan_udf_bg_complete"); v != nil {
		namespaceInfo.ScanUdfBgComplete = v
	}
	if v := parseInt(infoMap, "scan_udf_bg_error"); v != nil {
		namespaceInfo.ScanUdfBgError = v
	}

	return &namespaceInfo
}

// parseNodeStats creates NodeStats using values present in the statistics map
func parseNodeStats(stats map[string]string) *NodeStats {
	nodeStats := NodeStats{}

	if v := parseInt(stats, "client_connections"); v != nil {
		nodeStats.ClientConnections = v
	}
	if v := parseInt(stats, "fabric_connections"); v != nil {
		nodeStats.FabricConnections = v
	}
	if v := parseInt(stats, "heartbeat_connections"); v != nil {
		nodeStats.HeartbeatConnections = v
	}

	if v := parseInt(stats, "client_connections_closed"); v != nil {
		nodeStats.ClientConnectionsClosed = v
	}
	if v := parseInt(stats, "client_connections_opened"); v != nil {
		nodeStats.ClientConnectionsOpened = v
	}
	if v := parseInt(stats, "fabric_connections_closed"); v != nil {
		nodeStats.FabricConnectionsClosed = v
	}
	if v := parseInt(stats, "fabric_connections_opened"); v != nil {
		nodeStats.FabricConnectionsOpened = v
	}
	if v := parseInt(stats, "heartbeat_connections_closed"); v != nil {
		nodeStats.HeartbeatConnectionsClosed = v
	}
	if v := parseInt(stats, "heartbeat_connections_opened"); v != nil {
		nodeStats.HeartbeatConnectionsOpened = v
	}

	if v := parseInt(stats, "system_free_mem_pct"); v != nil {
		nodeStats.SystemFreeMemPct = v
	}

	return &nodeStats
}

func NamespaceKey(namespace string) string {
	return "namespace/" + namespace
}

// parseInt parses a string value as int64 if present.
// If the key isn't present or parsing fails, nil is returned
func parseInt(i InfoResponse, key string) *int64 {
	if v, ok := i[key]; ok {
		if intV, err := strconv.ParseInt(v, 10, 64); err == nil {
			return &intV
		}
	}
	return nil
}

// valueToMap transforms a separated list of values into a map
// i is the InfoResponse, rootKey is the key of the field we want to transform
func valueToMap(i InfoResponse, rootKey string) map[string]string {
	values := make(map[string]string)
	s, ok := i[rootKey]
	if !ok {
		return values
	}

	// s looks like `failed_best_practices=true;cluster_size=3;cluster_key=156A4BCA01E3;...`
	for _, kv := range splitFields(s) {
		parts := strings.Split(kv, "=")
		// TODO: Warn/indicate we skipped a pair
		if len(parts) != 2 {
			continue
		}
		values[parts[0]] = parts[1]
	}

	return values
}

func splitFields(info string) []string {
	return strings.Split(info, fieldSeparator)
}
