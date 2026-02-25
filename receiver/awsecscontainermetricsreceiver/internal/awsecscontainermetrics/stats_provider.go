// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import (
	"encoding/json"
	"fmt"

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
