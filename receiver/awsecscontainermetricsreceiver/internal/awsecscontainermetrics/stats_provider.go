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

// StatsProvider wraps a RestClient, returning an unmarshaled metadata and docker stats
type StatsProvider struct {
	rc RestClient
}

// NewStatsProvider returns a new stats provider
func NewStatsProvider(rc RestClient) *StatsProvider {
	return &StatsProvider{rc: rc}
}

// GetStats calls the ecs task metadata endpoint and unmarshals the data
func (p *StatsProvider) GetStats() (map[string]*ContainerStats, TaskMetadata, error) {
	stats := make(map[string]*ContainerStats)
	var metadata TaskMetadata

	taskStats, taskMetadata, err := p.rc.EndpointResponse()
	if err != nil {
		return stats, metadata, fmt.Errorf("cannot read data from task metadata endpoint: %w", err)
	}

	err = json.Unmarshal(taskStats, &stats)
	if err != nil {
		return stats, metadata, fmt.Errorf("cannot unmarshall task stats: %w", err)
	}

	err = json.Unmarshal(taskMetadata, &metadata)
	if err != nil {
		return stats, metadata, fmt.Errorf("cannot unmarshall task metadata: %w", err)
	}
	return stats, metadata, nil
}
