package azure

import (
	_ "embed"
	"encoding/json"
)

//go:embed attribute_name_mappings/resource_logs.json
var resource_logs_json []byte
var resource_logs_map = unmarshal(resource_logs_json)

var mappings = map[string]map[string]string{
	"common": resource_logs_map,
}

func unmarshal(data []byte) map[string]string {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
	}
	return m
}
