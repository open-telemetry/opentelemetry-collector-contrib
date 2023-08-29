// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

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
}

func (s *ServiceConfig) validate() error {
	_, err := s.newMatcher(matcherOptions{})
	return err
}

func (s *ServiceConfig) newMatcher(opts matcherOptions) (targetMatcher, error) {
	if s.NamePattern == "" {
		return nil, fmt.Errorf("name_pattern is empty")
	}

	nameRegex, err := regexp.Compile(s.NamePattern)
	if err != nil {
		return nil, fmt.Errorf("invalid name pattern %w", err)
	}
	var containerRegex *regexp.Regexp
	if s.ContainerNamePattern != "" {
		containerRegex, err = regexp.Compile(s.ContainerNamePattern)
		if err != nil {
			return nil, fmt.Errorf("invalid container name pattern %w", err)
		}
	}
	expSetting, err := s.newExportSetting()
	if err != nil {
		return nil, err
	}
	return &serviceMatcher{
		logger:             opts.Logger,
		cfg:                *s,
		nameRegex:          nameRegex,
		containerNameRegex: containerRegex,
		exportSetting:      expSetting,
	}, nil
}

func serviceConfigsToMatchers(cfgs []ServiceConfig) []matcherConfig {
	matchers := make([]matcherConfig, len(cfgs))
	for i, cfg := range cfgs {
		// NOTE: &cfg points to the temp var, whose value would end up be the last one in the slice.
		copied := cfg
		matchers[i] = &copied
	}
	return matchers
}

type serviceMatcher struct {
	logger    *zap.Logger
	cfg       ServiceConfig
	nameRegex *regexp.Regexp
	// can be nil, which means matching all the container in the task (whose service name is matched by nameRegex)
	containerNameRegex *regexp.Regexp
	exportSetting      *commonExportSetting
}

func (s *serviceMatcher) matcherType() matcherType {
	return matcherTypeService
}

func (s *serviceMatcher) matchTargets(t *taskAnnotated, c *ecs.ContainerDefinition) ([]matchedTarget, error) {
	// Service info is only attached for tasks whose services are included in config.
	// However, Match is called on tasks so we need to guard nil pointer.
	if t.Service == nil {
		return nil, errNotMatched
	}
	if !s.nameRegex.MatchString(aws.StringValue(t.Service.ServiceName)) {
		return nil, errNotMatched
	}
	// The rest is same as taskDefinitionMatcher
	return matchContainerByName(s.containerNameRegex, s.exportSetting, c)
}

// serviceConfigsToFilter reduce number of describe service API call
func serviceConfigsToFilter(cfgs []ServiceConfig) (serviceNameFilter, error) {
	// If no service config, don't describe any services
	if len(cfgs) == 0 {
		return func(name string) bool {
			return false
		}, nil
	}
	regs := make([]*regexp.Regexp, len(cfgs))
	for i, cfg := range cfgs {
		r, err := regexp.Compile(cfg.NamePattern)
		if err != nil {
			return nil, fmt.Errorf("invalid service name pattern %q: %w", cfg.NamePattern, err)
		}
		regs[i] = r
	}
	return func(name string) bool {
		for _, r := range regs {
			if r.MatchString(name) {
				return true
			}
		}
		return false
	}, nil
}
