// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecssd

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"go.uber.org/zap"
)

type ServiceDiscovery struct {
	logger   *zap.Logger
	cfg      Config
	fetcher  Fetcher
	filter   *TaskFilter
	exporter *TaskExporter
}

type ServiceDiscoveryOptions struct {
	Logger          *zap.Logger
	FetcherOverride Fetcher // for injecting MockFetcher
}

func New(cfg Config, opts ServiceDiscoveryOptions) (*ServiceDiscovery, error) {
	serviceNameFilter, err := serviceConfigsToFilter(cfg.Services)
	if err != nil {
		return nil, fmt.Errorf("init serivce name filter failed: %w", err)
	}
	var fetcher Fetcher
	if opts.FetcherOverride != nil {
		fetcher = opts.FetcherOverride
	} else {
		fetcher, err = NewTaskFetcher(TaskFetcherOptions{
			Logger:            opts.Logger,
			Region:            cfg.ClusterRegion,
			Cluster:           cfg.ClusterName,
			ServiceNameFilter: serviceNameFilter,
		})
		if err != nil {
			return nil, fmt.Errorf("init fetcher failed: %w", err)
		}
	}
	matchers, err := newMatchers(cfg, MatcherOptions{
		Logger: opts.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("init matchers failed: %w", err)
	}
	filter, err := NewTaskFilter(cfg, TaskFilterOptions{Logger: opts.Logger, Matchers: matchers})
	if err != nil {
		return nil, fmt.Errorf("init filter failed: %w", err)
	}
	exporter := NewTaskExporter(TaskExporterOptions{
		Logger:   opts.Logger,
		Matchers: matchers,
		Cluster:  cfg.ClusterName,
	})
	return &ServiceDiscovery{
		logger:   opts.Logger,
		cfg:      cfg,
		fetcher:  fetcher,
		filter:   filter,
		exporter: exporter,
	}, nil
}

// RunAndWriteFile writes the output to Config.ResultFile.
func (s *ServiceDiscovery) RunAndWriteFile(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.RefreshInterval)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			targets, err := s.Discover(ctx)
			if err != nil {
				return err
			}
			b, err := TargetsToFileSDYAML(targets, s.cfg.JobLabelName)
			if err != nil {
				return err
			}
			if err := ioutil.WriteFile(s.cfg.ResultFile, b, 0600); err != nil {
				return err
			}
		}
	}
}

func (s *ServiceDiscovery) Discover(ctx context.Context) ([]PrometheusECSTarget, error) {
	tasks, err := s.fetcher.FetchAndDecorate(ctx)
	if err != nil {
		return nil, err
	}
	filtered, err := s.filter.Filter(tasks)
	if err != nil {
		return nil, err
	}
	exported, err := s.exporter.ExportTasks(filtered)
	if err != nil {
		return nil, err
	}
	return exported, nil
}
