// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"go.uber.org/zap"
)

type TaskDefinitionConfig struct {
	CommonExporterConfig `mapstructure:",squash" yaml:",inline"`

	// ArnPattern is mandatory, empty string means arn based match is skipped.
	ArnPattern string `mapstructure:"arn_pattern" yaml:"arn_pattern"`
	// ContainerNamePattern is optional, empty string means all containers in that task definition would be exported.
	// Otherwise both service and container name patterns need to match.
	ContainerNamePattern string `mapstructure:"container_name_pattern" yaml:"container_name_pattern"`
}

func (t *TaskDefinitionConfig) validate() error {
	_, err := t.newMatcher(matcherOptions{})
	return err
}

func (t *TaskDefinitionConfig) newMatcher(opts matcherOptions) (targetMatcher, error) {
	if t.ArnPattern == "" {
		return nil, errors.New("arn_pattern is empty")
	}

	arnRegex, err := regexp.Compile(t.ArnPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid arn pattern %w", err)
	}
	var containerRegex *regexp.Regexp
	if t.ContainerNamePattern != "" {
		containerRegex, err = regexp.Compile(t.ContainerNamePattern)
		if err != nil {
			return nil, fmt.Errorf("invalid container name pattern %w", err)
		}
	}
	expSetting, err := t.newExportSetting()
	if err != nil {
		return nil, err
	}
	return &taskDefinitionMatcher{
		logger:             opts.Logger,
		cfg:                *t,
		arnRegex:           arnRegex,
		containerNameRegex: containerRegex,
		exportSetting:      expSetting,
	}, nil
}

func taskDefinitionConfigsToMatchers(cfgs []TaskDefinitionConfig) []matcherConfig {
	matchers := make([]matcherConfig, len(cfgs))
	for i, cfg := range cfgs {
		// NOTE: &cfg points to the temp var, whose value would end up be the last one in the slice.
		copied := cfg
		matchers[i] = &copied
	}
	return matchers
}

type taskDefinitionMatcher struct {
	logger *zap.Logger
	cfg    TaskDefinitionConfig
	// should never be nil because Init must reject it and caller should stop
	arnRegex *regexp.Regexp
	// if nil, matches all the container in the task (whose task definition name is matched by arnRegex)
	containerNameRegex *regexp.Regexp
	exportSetting      *commonExportSetting
}

func (m *taskDefinitionMatcher) matcherType() matcherType {
	return matcherTypeTaskDefinition
}

func (m *taskDefinitionMatcher) matchTargets(t *taskAnnotated, c ecstypes.ContainerDefinition) ([]matchedTarget, error) {
	// Check arn
	if !m.arnRegex.MatchString(aws.ToString(t.Task.TaskDefinitionArn)) {
		return nil, errNotMatched
	}
	// The rest is same as ServiceMatcher
	return matchContainerByName(m.containerNameRegex, m.exportSetting, c)
}
