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
	"fmt"

	"github.com/aws/aws-sdk-go/service/ecs"
)

type ServiceConfig struct {
	CommonExporterConfig `mapstructure:",squash" yaml:",inline"`

	// NamePattern is mandetory, empty string means service name based match is skipped.
	NamePattern string `mapstructure:"name_pattern" yaml:"name_pattern"`
	// ContainerNamePattern is optional, empty string means all containers in that service would be exported.
	// Otherwise both service and container name petterns need to metch.
	ContainerNamePattern string `mapstructure:"container_name_pattern" yaml:"container_name_pattern"`
}

func (s *ServiceConfig) Init() error {
	panic("not implemented")
}

func (s *ServiceConfig) NewMatcher(opts MatcherOptions) (Matcher, error) {
	panic("not implemented")
}

type ServiceNameFilter func(name string) bool

func serviceConfigsToFilter(cfgs []ServiceConfig) (ServiceNameFilter, error) {
	// If no service config, don't descibe any services
	if len(cfgs) == 0 {
		return func(name string) bool {
			return false
		}, nil
	}
	return nil, fmt.Errorf("not implemented")
}

type ServiceMatcher struct {
	cfg ServiceConfig
}

func (s *ServiceMatcher) Type() MatcherType {
	return MatcherTypeService
}

func (s *ServiceMatcher) ExporterConfig() CommonExporterConfig {
	return s.cfg.CommonExporterConfig
}

func (s *ServiceMatcher) MatchTargets(t *Task, c *ecs.ContainerDefinition) ([]MatchedTarget, error) {
	panic("not implemented")
}
