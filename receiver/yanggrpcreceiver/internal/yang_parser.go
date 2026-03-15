// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Helper function to create int64 pointers
func int64Ptr(v int64) *int64 {
	return &v
}

// YANGDataType represents the YANG data type information
type YANGDataType struct {
	Type        string           `json:"type"`        // uint8, uint16, uint32, uint64, int8, int16, int32, int64, string, boolean, decimal64, etc.
	Units       string           `json:"units"`       // units like "percent", "seconds", "bytes", "packets"
	Range       *YANGRange       `json:"range"`       // min/max values if applicable
	Description string           `json:"description"` // field description
	Enumeration map[string]int64 `json:"enumeration"` // for enum types: name -> value
}

// YANGRange represents min/max constraints for numeric types
type YANGRange struct {
	Min *int64 `json:"min"`
	Max *int64 `json:"max"`
}

// YANGModule represents a parsed YANG module with its key information
type YANGModule struct {
	Name        string                   `json:"name"`
	Namespace   string                   `json:"namespace"`
	Prefix      string                   `json:"prefix"`
	KeyedLeafs  map[string]string        `json:"keyed_leafs"` // path -> key field name
	ListKeys    map[string][]string      `json:"list_keys"`   // list path -> key fields
	DataTypes   map[string]*YANGDataType `json:"data_types"`  // field path -> data type info
	Description string                   `json:"description"`
}

// YANGParser handles parsing of YANG modules to identify keyed elements
type YANGParser struct {
	modules map[string]*YANGModule
}

// NewYANGParser creates a new YANG parser instance
func NewYANGParser() *YANGParser {
	return &YANGParser{
		modules: make(map[string]*YANGModule),
	}
}

// LoadBuiltinModules loads pre-analyzed YANG modules for Cisco IOS XE 17.18.1
func (p *YANGParser) LoadBuiltinModules() {
	// Cisco-IOS-XE-interfaces-oper module based on analysis
	interfacesModule := &YANGModule{
		Name:      "Cisco-IOS-XE-interfaces-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-interfaces-oper",
		Prefix:    "interfaces-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/interfaces-ios-xe-oper:interfaces/interface": "name",
			"/interfaces/interface":                        "name",
		},
		ListKeys: map[string][]string{
			"/interfaces-ios-xe-oper:interfaces/interface": {"name"},
			"/interfaces/interface":                        {"name"},
		},
		DataTypes: map[string]*YANGDataType{
			// Interface key fields
			"/interfaces/interface/name":           {Type: "string", Description: "Interface name identifier"},
			"/interfaces/interface/interface-type": {Type: "identityref", Description: "Interface type identity"},

			// Counter fields - 64-bit unsigned integers
			"/interfaces/interface/statistics/in-octets":         {Type: "uint64", Units: "bytes", Description: "Total bytes received"},
			"/interfaces/interface/statistics/in-unicast-pkts":   {Type: "uint64", Units: "packets", Description: "Unicast packets received"},
			"/interfaces/interface/statistics/in-broadcast-pkts": {Type: "uint64", Units: "packets", Description: "Broadcast packets received"},
			"/interfaces/interface/statistics/in-multicast-pkts": {Type: "uint64", Units: "packets", Description: "Multicast packets received"},
			"/interfaces/interface/statistics/in-discards":       {Type: "uint32", Units: "packets", Description: "Inbound packets discarded"},
			"/interfaces/interface/statistics/in-errors":         {Type: "uint32", Units: "packets", Description: "Inbound packets with errors"},
			"/interfaces/interface/statistics/in-unknown-protos": {Type: "uint32", Units: "packets", Description: "Inbound packets with unknown protocols"},

			"/interfaces/interface/statistics/out-octets":         {Type: "uint64", Units: "bytes", Description: "Total bytes transmitted"},
			"/interfaces/interface/statistics/out-unicast-pkts":   {Type: "uint64", Units: "packets", Description: "Unicast packets transmitted"},
			"/interfaces/interface/statistics/out-broadcast-pkts": {Type: "uint64", Units: "packets", Description: "Broadcast packets transmitted"},
			"/interfaces/interface/statistics/out-multicast-pkts": {Type: "uint64", Units: "packets", Description: "Multicast packets transmitted"},
			"/interfaces/interface/statistics/out-discards":       {Type: "uint32", Units: "packets", Description: "Outbound packets discarded"},
			"/interfaces/interface/statistics/out-errors":         {Type: "uint32", Units: "packets", Description: "Outbound packets with errors"},

			// Rate fields - 32-bit unsigned integers
			"/interfaces/interface/statistics/rx-pps":  {Type: "uint32", Units: "packets-per-second", Description: "Receive packet rate"},
			"/interfaces/interface/statistics/rx-kbps": {Type: "uint32", Units: "kilobits-per-second", Description: "Receive bit rate"},
			"/interfaces/interface/statistics/tx-pps":  {Type: "uint32", Units: "packets-per-second", Description: "Transmit packet rate"},
			"/interfaces/interface/statistics/tx-kbps": {Type: "uint32", Units: "kilobits-per-second", Description: "Transmit bit rate"},

			// Other statistics
			"/interfaces/interface/statistics/num-flaps":          {Type: "uint32", Units: "count", Description: "Number of interface flaps"},
			"/interfaces/interface/statistics/in-crc-errors":      {Type: "uint32", Units: "packets", Description: "CRC error packets"},
			"/interfaces/interface/statistics/discontinuity-time": {Type: "yang:date-and-time", Description: "Time of last counter discontinuity"},

			// 64-bit versions of counters for high-speed interfaces
			"/interfaces/interface/statistics/in-octets-64":         {Type: "uint64", Units: "bytes", Description: "64-bit inbound byte counter"},
			"/interfaces/interface/statistics/out-octets-64":        {Type: "uint64", Units: "bytes", Description: "64-bit outbound byte counter"},
			"/interfaces/interface/statistics/in-discards-64":       {Type: "uint64", Units: "packets", Description: "64-bit inbound discard counter"},
			"/interfaces/interface/statistics/in-errors-64":         {Type: "uint64", Units: "packets", Description: "64-bit inbound error counter"},
			"/interfaces/interface/statistics/in-unknown-protos-64": {Type: "uint64", Units: "packets", Description: "64-bit unknown protocol counter"},
		},
		Description: "Interface operational data for Cisco IOS XE devices",
	}

	// Add more common Cisco modules based on known patterns
	bgpModule := &YANGModule{
		Name:      "Cisco-IOS-XE-bgp-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-bgp-oper",
		Prefix:    "bgp-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/bgp-state-data/neighbors/neighbor":              "neighbor-id",
			"/bgp-state-data/address-families/address-family": "afi-safi",
		},
		ListKeys: map[string][]string{
			"/bgp-state-data/neighbors/neighbor":              {"neighbor-id"},
			"/bgp-state-data/address-families/address-family": {"afi-safi"},
		},
		Description: "BGP operational data for Cisco IOS XE devices",
	}

	// Cisco-IOS-XE-process-cpu-oper module based on your screenshot
	processCPUModule := &YANGModule{
		Name:      "Cisco-IOS-XE-process-cpu-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-process-cpu-oper",
		Prefix:    "process-cpu-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/cpu-usage/cpu-utilization/cpu-usage-processes": "name",
		},
		ListKeys: map[string][]string{
			"/cpu-usage/cpu-utilization/cpu-usage-processes": {"name"},
		},
		DataTypes: map[string]*YANGDataType{
			// Process identification
			"/cpu-usage/cpu-utilization/cpu-usage-processes/name": {Type: "string", Description: "Process name"},

			// CPU utilization percentages - based on your screenshot showing uint8, 0-255 range, percent units
			"/cpu-usage/cpu-utilization/five-seconds":      {Type: "uint8", Units: "percent", Range: &YANGRange{Min: int64Ptr(0), Max: int64Ptr(255)}, Description: "CPU busy percentage in last 5-seconds"},
			"/cpu-usage/cpu-utilization/five-seconds-intr": {Type: "uint8", Units: "percent", Range: &YANGRange{Min: int64Ptr(0), Max: int64Ptr(255)}, Description: "CPU interrupt percentage in last 5-seconds"},
			"/cpu-usage/cpu-utilization/one-minute":        {Type: "uint8", Units: "percent", Range: &YANGRange{Min: int64Ptr(0), Max: int64Ptr(255)}, Description: "CPU busy percentage in last minute"},
			"/cpu-usage/cpu-utilization/five-minutes":      {Type: "uint8", Units: "percent", Range: &YANGRange{Min: int64Ptr(0), Max: int64Ptr(255)}, Description: "CPU busy percentage in last 5-minutes"},
		},
		Description: "Process CPU utilization operational data for Cisco IOS XE devices",
	}

	ospfModule := &YANGModule{
		Name:      "Cisco-IOS-XE-ospf-oper",
		Namespace: "http://cisco.com/ns/yang/Cisco-IOS-XE-ospf-oper",
		Prefix:    "ospf-ios-xe-oper",
		KeyedLeafs: map[string]string{
			"/ospf-oper-data/ospf-state/ospf-instance":           "router-id",
			"/ospf-oper-data/ospf-state/ospf-instance/ospf-area": "area-id",
		},
		ListKeys: map[string][]string{
			"/ospf-oper-data/ospf-state/ospf-instance":           {"router-id"},
			"/ospf-oper-data/ospf-state/ospf-instance/ospf-area": {"area-id"},
		},
		DataTypes: map[string]*YANGDataType{
			"/ospf-oper-data/ospf-state/ospf-instance/router-id":         {Type: "inet:ipv4-address", Description: "OSPF router ID"},
			"/ospf-oper-data/ospf-state/ospf-instance/ospf-area/area-id": {Type: "uint32", Description: "OSPF area identifier"},
		},
		Description: "OSPF operational data for Cisco IOS XE devices",
	}

	p.modules[interfacesModule.Name] = interfacesModule
	p.modules[bgpModule.Name] = bgpModule
	p.modules[processCPUModule.Name] = processCPUModule
	p.modules[ospfModule.Name] = ospfModule

	log.Printf("Loaded %d builtin YANG modules", len(p.modules))
}

// GetKeyForPath returns the key field name for a given YANG path
func (p *YANGParser) GetKeyForPath(moduleName, path string) string {
	module, exists := p.modules[moduleName]
	if !exists {
		return ""
	}

	// Try exact match first
	if key, found := module.KeyedLeafs[path]; found {
		return key
	}

	// Try pattern matching for flexible path matching
	for yangPath, key := range module.KeyedLeafs {
		if p.matchPath(yangPath, path) {
			return key
		}
	}

	return ""
}

// GetKeysForList returns all key fields for a YANG list
func (p *YANGParser) GetKeysForList(moduleName, listPath string) []string {
	module, exists := p.modules[moduleName]
	if !exists {
		return nil
	}

	// Try exact match first
	if keys, found := module.ListKeys[listPath]; found {
		return keys
	}

	// Try pattern matching
	for yangPath, keys := range module.ListKeys {
		if p.matchPath(yangPath, listPath) {
			return keys
		}
	}

	return nil
}

// matchPath performs flexible path matching
func (p *YANGParser) matchPath(yangPath, telemetryPath string) bool {
	// Remove prefixes for comparison
	cleanYang := p.removePrefixes(yangPath)
	cleanTelemetry := p.removePrefixes(telemetryPath)

	// Direct match
	if cleanYang == cleanTelemetry {
		return true
	}

	// Pattern match - check if telemetry path ends with yang path
	if strings.HasSuffix(cleanTelemetry, cleanYang) {
		return true
	}

	// Pattern match - check if yang path pattern matches telemetry
	return p.isPathPattern(cleanYang, cleanTelemetry)
}

// removePrefixes removes YANG prefixes from paths
func (*YANGParser) removePrefixes(path string) string {
	// Remove common prefixes like "interfaces-ios-xe-oper:"
	re := regexp.MustCompile(`[a-zA-Z0-9-]+:`)
	return re.ReplaceAllString(path, "")
}

// isPathPattern checks if a YANG path pattern matches a telemetry path
func (*YANGParser) isPathPattern(yangPattern, telemetryPath string) bool {
	// Simple pattern matching - can be enhanced
	yangParts := strings.Split(strings.Trim(yangPattern, "/"), "/")
	telemetryParts := strings.Split(strings.Trim(telemetryPath, "/"), "/")

	if len(yangParts) > len(telemetryParts) {
		return false
	}

	// Check if yang pattern matches end of telemetry path
	offset := len(telemetryParts) - len(yangParts)
	for i, yangPart := range yangParts {
		if telemetryParts[offset+i] != yangPart {
			return false
		}
	}

	return true
}

// AnalyzeEncodingPath analyzes a telemetry encoding path to identify keys
func (p *YANGParser) AnalyzeEncodingPath(encodingPath string) *PathAnalysis {
	analysis := &PathAnalysis{
		EncodingPath: encodingPath,
		ModuleName:   "",
		Keys:         make(map[string]string),
		ListPath:     "",
	}

	// Extract module name from encoding path
	// Format: Cisco-IOS-XE-interfaces-oper:interfaces/interface/statistics
	parts := strings.Split(encodingPath, ":")
	if len(parts) >= 2 {
		analysis.ModuleName = parts[0]
		pathPart := parts[1]

		// Find the list path (usually ends before /statistics, /state, etc.)
		pathSegments := strings.Split(pathPart, "/")
		if len(pathSegments) >= 2 {
			// For interface statistics: interfaces/interface/statistics -> interfaces/interface
			if pathSegments[len(pathSegments)-1] == "statistics" && len(pathSegments) >= 2 {
				analysis.ListPath = "/" + strings.Join(pathSegments[:len(pathSegments)-1], "/")
			} else {
				analysis.ListPath = "/" + strings.Join(pathSegments, "/")
			}

			// Get key information for this path
			key := p.GetKeyForPath(analysis.ModuleName, analysis.ListPath)
			if key != "" {
				analysis.Keys[analysis.ListPath] = key
			}
		}
	}

	return analysis
}

// PathAnalysis contains the results of analyzing a telemetry path
type PathAnalysis struct {
	EncodingPath string            `json:"encoding_path"`
	ModuleName   string            `json:"module_name"`
	Keys         map[string]string `json:"keys"`      // path -> key field name
	ListPath     string            `json:"list_path"` // the list container path
}

// GetDataTypeForField returns the YANG data type information for a specific field
func (p *YANGParser) GetDataTypeForField(moduleName, fieldPath string) *YANGDataType {
	module, exists := p.modules[moduleName]
	if !exists {
		return nil
	}

	// Try exact match first
	if dataType, found := module.DataTypes[fieldPath]; found {
		return dataType
	}

	// Try pattern matching for flexible path matching
	for yangPath, dataType := range module.DataTypes {
		if p.matchPath(yangPath, fieldPath) {
			return dataType
		}
	}

	return nil
}

// GetDataTypeForEncodingPath analyzes an encoding path and field name to get data type
func (p *YANGParser) GetDataTypeForEncodingPath(encodingPath, fieldName string) *YANGDataType {
	analysis := p.AnalyzeEncodingPath(encodingPath)
	if analysis == nil {
		return nil
	}

	// Construct possible field paths
	possiblePaths := []string{
		analysis.ListPath + "/" + fieldName,
		analysis.ListPath + "/statistics/" + fieldName,
		fieldName, // Direct field name
	}

	for _, path := range possiblePaths {
		if dataType := p.GetDataTypeForField(analysis.ModuleName, path); dataType != nil {
			return dataType
		}
	}

	return nil
}

// IsNumericType checks if a YANG data type is numeric
func (dt *YANGDataType) IsNumericType() bool {
	if dt == nil {
		return false
	}

	numericTypes := []string{
		"uint8", "uint16", "uint32", "uint64",
		"int8", "int16", "int32", "int64",
		"decimal64",
	}

	return slices.Contains(numericTypes, dt.Type)
}

// IsCounterType checks if this is a counter-type metric (monotonically increasing)
func (dt *YANGDataType) IsCounterType() bool {
	if dt == nil {
		return false
	}

	// First check if it's a rate/gauge type - rates are NOT counters
	if dt.IsGaugeType() {
		return false
	}

	// Counter types are typically uint64 with units like bytes, packets, etc.
	// But NOT rate units (which are handled as gauges)
	if strings.HasPrefix(dt.Type, "uint") {
		counterUnits := []string{"bytes", "packets", "count", "errors", "discards"}
		if slices.Contains(counterUnits, dt.Units) {
			return true
		}
	}

	return false
}

// IsGaugeType checks if this is a gauge-type metric (can increase or decrease)
func (dt *YANGDataType) IsGaugeType() bool {
	if dt == nil {
		return false
	}

	// Gauge types include rates, percentages, current values
	gaugeUnits := []string{
		"percent", "per-second", "pps", "bps", "kbps", "mbps", "gbps",
		"utilization", "rate", "current", "level",
	}

	for _, unit := range gaugeUnits {
		if strings.Contains(dt.Units, unit) {
			return true
		}
	}

	return false
}

// SaveModulesToFile saves the loaded modules to a JSON file for inspection
func (p *YANGParser) SaveModulesToFile(filename string) error {
	data, err := json.MarshalIndent(p.modules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal modules: %w", err)
	}

	return os.WriteFile(filename, data, 0o600)
}

// LoadModulesFromFile loads modules from a JSON file
func (p *YANGParser) LoadModulesFromFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	return json.Unmarshal(data, &p.modules)
}

// GetAvailableModules returns a list of loaded module names
func (p *YANGParser) GetAvailableModules() []string {
	var modules []string
	for name := range p.modules {
		modules = append(modules, name)
	}
	return modules
}

// ExtractYANGFromFiles attempts to extract YANG module information from .yang files
func (p *YANGParser) ExtractYANGFromFiles(yangDir string) error {
	return filepath.Walk(yangDir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".yang") {
			return nil
		}

		// Basic YANG file parsing - can be enhanced
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		module := p.parseYANGContent(string(content), filepath.Base(path))
		if module != nil {
			p.modules[module.Name] = module
		}

		return nil
	})
}

// parseYANGContent performs basic parsing of YANG file content
func (*YANGParser) parseYANGContent(content, _ string) *YANGModule {
	lines := strings.Split(content, "\n")
	module := &YANGModule{
		KeyedLeafs: make(map[string]string),
		ListKeys:   make(map[string][]string),
	}

	var currentPath []string
	var inList bool
	var listPath string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract module name
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				module.Name = strings.TrimSuffix(parts[1], " {")
			}
		}

		// Extract namespace
		if strings.HasPrefix(line, "namespace ") {
			re := regexp.MustCompile(`namespace\s+"([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				module.Namespace = matches[1]
			}
		}

		// Extract prefix
		if strings.HasPrefix(line, "prefix ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				module.Prefix = strings.Trim(parts[1], "\";")
			}
		}

		// Detect list definitions with keys
		if strings.Contains(line, "list ") && strings.Contains(line, "{") {
			inList = true
			re := regexp.MustCompile(`list\s+([^\s{]+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				currentPath = append(currentPath, matches[1])
				listPath = "/" + strings.Join(currentPath, "/")
			}
		}

		// Extract key information
		if inList && strings.HasPrefix(line, "key ") {
			re := regexp.MustCompile(`key\s+"([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				keys := strings.Fields(matches[1])
				module.ListKeys[listPath] = keys
				if len(keys) > 0 {
					module.KeyedLeafs[listPath] = keys[0] // Primary key
				}
			}
		}

		// Handle nesting and closing braces
		if strings.Contains(line, "}") {
			if inList {
				inList = false
				if len(currentPath) > 0 {
					currentPath = currentPath[:len(currentPath)-1]
				}
			}
		}
	}

	if module.Name == "" {
		return nil
	}

	return module
}
