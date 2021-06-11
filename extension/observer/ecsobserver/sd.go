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

package ecsobserver

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
	fetcher  *taskFetcher
	filter   *taskFilter
	exporter *taskExporter
}

type ServiceDiscoveryOptions struct {
	Logger          *zap.Logger
	FetcherOverride *taskFetcher // for test
}

func NewDiscovery(cfg Config, opts ServiceDiscoveryOptions) (*ServiceDiscovery, error) {
	svcNameFilter, err := serviceConfigsToFilter(cfg.Services)
	if err != nil {
		return nil, fmt.Errorf("init serivce name filter failed: %w", err)
	}
	var fetcher *taskFetcher
	if opts.FetcherOverride != nil {
		fetcher = opts.FetcherOverride
	} else {
		fetcher, err = newTaskFetcher(taskFetcherOptions{
			Logger:            opts.Logger,
			Region:            cfg.ClusterRegion,
			Cluster:           cfg.ClusterName,
			serviceNameFilter: svcNameFilter,
		})
		if err != nil {
			return nil, fmt.Errorf("init fetcher failed: %w", err)
		}
	}
	matchers, err := newMatchers(cfg, MatcherOptions{Logger: opts.Logger})
	if err != nil {
		return nil, fmt.Errorf("init matchers failed: %w", err)
	}
	filter := newTaskFilter(opts.Logger, matchers)
	exporter := newTaskExporter(opts.Logger, cfg.ClusterName)
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
				// FIXME: better error handling here, for now just log and continue
				s.logger.Error("Discover failed", zap.Error(err))
				continue
			}

			// Encoding and file write error should never happen,
			// so we stop extension by returning error.
			b, err := targetsToFileSDYAML(targets, s.cfg.JobLabelName)
			if err != nil {
				return err
			}
			// NOTE: We assume the folder already exists and does NOT try to create one.
			if err := ioutil.WriteFile(s.cfg.ResultFile, b, 0600); err != nil {
				return err
			}
		}
	}
}

func (s *ServiceDiscovery) Discover(ctx context.Context) ([]PrometheusECSTarget, error) {
	tasks, err := s.fetcher.fetchAndDecorate(ctx)
	if err != nil {
		return nil, err
	}
	filtered, err := s.filter.filter(tasks)
	if err != nil {
		return nil, err
	}
	exported, err := s.exporter.exportTasks(filtered)
	if err != nil {
		return nil, err
	}
	return exported, nil
}
