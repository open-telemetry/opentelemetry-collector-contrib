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
	"time"

	"go.uber.org/zap"
)

type ServiceDiscovery struct {
	logger *zap.Logger
	cfg    Config
}

type ServiceDiscoveryOptions struct {
	Logger *zap.Logger
}

func NewDiscovery(cfg Config, opts ServiceDiscoveryOptions) (*ServiceDiscovery, error) {
	// NOTE: there are other init logic, currently removed to reduce pr size
	return &ServiceDiscovery{
		logger: opts.Logger,
		cfg:    cfg,
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
			// do actual work
		}
	}
}

func (s *ServiceDiscovery) Discover(ctx context.Context) ([]PrometheusECSTarget, error) {
	return nil, fmt.Errorf("not implemented")
}
