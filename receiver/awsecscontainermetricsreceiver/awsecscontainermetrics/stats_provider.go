// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetrics

import (
	"encoding/json"
	"fmt"
)

// StatsProvider wraps a RestClient, returning an unmarshaled
// stats.Summary struct from the kubelet API.
type StatsProvider struct {
	rc RestClient
}

func NewStatsProvider(rc RestClient) *StatsProvider {
	return &StatsProvider{rc: rc}
}

// GetStats calls the ecs task metadata endpoint and unmarshals the
// results into a stats.TaskStats struct.
func (p *StatsProvider) GetStats() (map[string]ContainerStats, TaskMetadata, error) {
	stats := make(map[string]ContainerStats)
	var metadata TaskMetadata

	taskStats, taskMetadata, err := p.rc.EndpointResponse()
	// fmt.Println(string(taskStats))
	if err != nil {
		return stats, metadata, err
	}
	//var out Model

	err = json.Unmarshal(taskStats, &stats)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return stats, metadata, err
	}

	err = json.Unmarshal(taskMetadata, &metadata)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return stats, metadata, err
	}
	return stats, metadata, nil
}
