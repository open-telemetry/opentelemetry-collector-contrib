// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import (
	"encoding/json"
	"fmt"
	"maps"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

// StatsProvider wraps a RestClient, returning an unmarshaled metadata and docker stats
type StatsProvider struct {
	rc               ecsutil.RestClient
	metadataProvider ecsutil.MetadataProvider
}

// NewStatsProvider returns a new stats provider
func NewStatsProvider(rc ecsutil.RestClient, logger *zap.Logger) *StatsProvider {
	return &StatsProvider{rc: rc, metadataProvider: ecsutil.NewTaskMetadataProvider(rc, logger)}
}

// GetStats calls the ecs task metadata endpoint and unmarshals the data
func (p *StatsProvider) GetStats() (map[string]*ContainerStats, ecsutil.TaskMetadata, error) {
	stats := make(map[string]*ContainerStats)
	var metadata ecsutil.TaskMetadata

	taskMetadata, err := p.metadataProvider.FetchTaskMetadata()
	if err != nil {
		return stats, metadata, fmt.Errorf("cannot read data from task metadata endpoint: %w", err)
	}

	if taskMetadata != nil {
		metadata = *taskMetadata
	}

	taskStats, err := p.rc.GetResponse(TaskStatsPath)
	if err != nil {
		return stats, metadata, fmt.Errorf("cannot read data from task metadata endpoint: %w", err)
	}

	err = json.Unmarshal(taskStats, &stats)
	if err != nil {
		return stats, metadata, fmt.Errorf("cannot unmarshall task stats: %w", err)
	}

	return stats, metadata, nil
}

// GetInstanceStats fetches stats and metadata for all tasks on the instance.
// This uses the /tasks/stats and /tasks endpoints available to Managed Daemon Services.
func (p *StatsProvider) GetInstanceStats() ([]TaskStatsEntry, error) {
	metadataResp, err := p.rc.GetResponse(InstanceMetadataPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read data from instance metadata endpoint: %w", err)
	}

	var allTaskMetadata []ecsutil.TaskMetadata
	err = json.Unmarshal(metadataResp, &allTaskMetadata)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal instance task metadata: %w", err)
	}

	statsResp, err := p.rc.GetResponse(InstanceStatsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read data from instance stats endpoint: %w", err)
	}

	var rawStats []map[string]*ContainerStats
	err = json.Unmarshal(statsResp, &rawStats)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal instance stats: %w", err)
	}

	// Flatten the array of single-entry maps into one map keyed by container ID
	allStats := make(map[string]*ContainerStats)
	for _, entry := range rawStats {
		maps.Copy(allStats, entry)
	}

	// Build per-task results pairing metadata with its container stats
	results := make([]TaskStatsEntry, 0, len(allTaskMetadata))
	for i := range allTaskMetadata {
		taskStats := make(map[string]*ContainerStats)
		for j := range allTaskMetadata[i].Containers {
			id := allTaskMetadata[i].Containers[j].DockerID
			if s, ok := allStats[id]; ok {
				taskStats[id] = s
			}
		}
		results = append(results, TaskStatsEntry{
			Stats:    taskStats,
			Metadata: allTaskMetadata[i],
		})
	}

	return results, nil
}

// TaskStatsEntry pairs a task's metadata with the container stats for that task.
type TaskStatsEntry struct {
	Stats    map[string]*ContainerStats
	Metadata ecsutil.TaskMetadata
}
