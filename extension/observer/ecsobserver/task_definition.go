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

type TaskDefinitionConfig struct {
	CommonExporterConfig `mapstructure:",squash" yaml:",inline"`

	// ArnPattern is mandetory, empty string means arn based match is skipped.
	ArnPattern string `mapstructure:"arn_pattern" yaml:"arn_pattern"`
	// ContainerNamePattern is optional, empty string means all containers in that task definition would be exported.
	// Otherwise both service and container name petterns need to metch.
	ContainerNamePattern string `mapstructure:"container_name_pattern" yaml:"container_name_pattern"`

	// should never be nil because Init must reject it and caller should stop
	arnRegex *regexp.Regexp
	// if nil, matches all the container in the task (whose task definition name is matched by arnRegex)
	containerNameRegex *regexp.Regexp
}

func (t *TaskDefinitionConfig) Init() error {
	if t.ArnPattern == "" {
		return fmt.Errorf("arn_pattern is empty")
	}

	r, err := regexp.Compile(t.ArnPattern)
	if err != nil {
		return fmt.Errorf("invalid arn pattern %w", err)
	}
	t.arnRegex = r
	if t.ContainerNamePattern != "" {
		r, err = regexp.Compile(t.ContainerNamePattern)
		if err != nil {
			return fmt.Errorf("invalid container name pattern %w", err)
		}
		t.containerNameRegex = r
	}
	return nil
}

func (t *TaskDefinitionConfig) NewMatcher(opts MatcherOptions) (Matcher, error) {
	return &taskDefinitionMatcher{
		logger: opts.Logger,
		cfg:    *t,
	}, nil
}

type taskDefinitionMatcher struct {
	logger *zap.Logger
	cfg    TaskDefinitionConfig
}

func (m *taskDefinitionMatcher) Type() MatcherType {
	return MatcherTypeTaskDefinition
}

func (m *taskDefinitionMatcher) MatchTargets(t *Task, c *ecs.ContainerDefinition) ([]MatchedTarget, error) {
	// Check arn
	if !m.cfg.arnRegex.MatchString(aws.StringValue(t.Task.TaskDefinitionArn)) {
		return nil, errNotMatched
	}
	// The rest is same as ServiceMatcher
	return matchContainerByName(m.cfg.containerNameRegex, m.cfg.CommonExporterConfig, c)
}
