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
	"github.com/aws/aws-sdk-go/service/ecs"
)

type TaskDefinitionConfig struct {
	CommonExporterConfig `mapstructure:",squash" yaml:",inline"`

	// ArnPattern is mandetory, empty string means arn based match is skipped.
	ArnPattern string `mapstructure:"arn_pattern" yaml:"arn_pattern"`
	// ContainerNamePattern is optional, empty string means all containers in that task definition would be exported.
	// Otherwise both service and container name petterns need to metch.
	ContainerNamePattern string `mapstructure:"container_name_pattern" yaml:"container_name_pattern"`
}

func (t *TaskDefinitionConfig) Init() error {
	panic("not implemented")
}

func (t *TaskDefinitionConfig) NewMatcher(opts MatcherOptions) (Matcher, error) {
	panic("not implemented")
}

type TaskDefinitionMatcher struct {
	cfg TaskDefinitionConfig
}

func (m *TaskDefinitionMatcher) Type() MatcherType {
	return MatcherTypeTaskDefinition
}

func (m *TaskDefinitionMatcher) ExporterConfig() CommonExporterConfig {
	return m.cfg.CommonExporterConfig
}

func (m *TaskDefinitionMatcher) MatchTargets(t *Task, c *ecs.ContainerDefinition) ([]MatchedTarget, error) {
	panic("not implemented")
}
