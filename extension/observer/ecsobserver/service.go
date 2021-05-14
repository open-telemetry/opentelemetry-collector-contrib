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
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"
)

type ServiceConfig struct {
	CommonExporterConfig `mapstructure:",squash" yaml:",inline"`

	// NamePattern is mandatory.
	NamePattern string `mapstructure:"name_pattern" yaml:"name_pattern"`
	// ContainerNamePattern is optional, empty string means all containers in that service would be exported.
	// Otherwise both service and container name petterns need to metch.
	ContainerNamePattern string `mapstructure:"container_name_pattern" yaml:"container_name_pattern"`

	// should never be nil because Init must reject it and caller should stop
	nameRegex *regexp.Regexp
	// if nil, matches all the container in the task (whose service name is matched by nameRegex)
	containerNameRegex *regexp.Regexp
}

func (s *ServiceConfig) Init() error {
	if s.NamePattern == "" {
		return fmt.Errorf("name_pattern is empty")
	}

	r, err := regexp.Compile(s.NamePattern)
	if err != nil {
		return fmt.Errorf("invalid name pattern %w", err)
	}
	s.nameRegex = r
	if s.ContainerNamePattern != "" {
		r, err = regexp.Compile(s.ContainerNamePattern)
		if err != nil {
			return fmt.Errorf("invalid container name pattern %w", err)
		}
		s.containerNameRegex = r
	}
	return nil
}

func (s *ServiceConfig) NewMatcher(opts MatcherOptions) Matcher {
	return &serviceMatcher{
		logger: opts.Logger,
		cfg:    *s,
	}
}

func serviceConfigsToMatchers(cfgs []ServiceConfig) []MatcherConfig {
	var matchers []MatcherConfig
	for _, cfg := range cfgs {
		// NOTE: &cfg points to the temp var, whose value would end up be the last one in the slice.
		copied := cfg
		matchers = append(matchers, &copied)
	}
	return matchers
}

type serviceMatcher struct {
	logger *zap.Logger
	cfg    ServiceConfig
}

func (s *serviceMatcher) Type() MatcherType {
	return MatcherTypeService
}

func (s *serviceMatcher) MatchTargets(t *Task, c *ecs.ContainerDefinition) ([]MatchedTarget, error) {
	// Service info is only attached for tasks whose services are included in config.
	// However, Match is called on tasks so we need to guard nil pointer.
	if t.Service == nil {
		return nil, errNotMatched
	}
	// nameRegex should never be nil
	if !s.cfg.nameRegex.MatchString(aws.StringValue(t.Service.ServiceName)) {
		return nil, errNotMatched
	}
	// The rest is same as taskDefinitionMatcher
	return matchContainerByName(s.cfg.containerNameRegex, s.cfg.CommonExporterConfig, c)
}
