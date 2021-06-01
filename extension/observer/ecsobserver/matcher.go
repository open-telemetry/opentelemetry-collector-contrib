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
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type Matcher interface {
	Type() MatcherType
	// MatchTargets returns targets fond from the specific container.
	// One container can have multiple targets because it may have multiple ports.
	MatchTargets(task *Task, container *ecs.ContainerDefinition) ([]MatchedTarget, error)
}

// matcherConfig should be implemented by all the matcher config structs
// for validation and initializing the actual matcher implementation.
type matcherConfig interface {
	// validate calls NewMatcher and only returns the error, it can be used in test
	// and the new config validator interface.
	validate() error
	// newMatcher validates config and creates a Matcher implementation.
	// The error is a config validation error
	newMatcher(options MatcherOptions) (Matcher, error)
}

type MatcherOptions struct {
	Logger *zap.Logger
}

type MatcherType int

// Values for enum MatcherType.
const (
	MatcherTypeService MatcherType = iota + 1
	MatcherTypeTaskDefinition
	MatcherTypeDockerLabel
)

func (t MatcherType) String() string {
	switch t {
	case MatcherTypeService:
		return "service"
	case MatcherTypeTaskDefinition:
		return "task_definition"
	case MatcherTypeDockerLabel:
		return "docker_label"
	default:
		// Give it a _matcher_type suffix so people can find it by string search.
		return "unknown_matcher_type"
	}
}

type MatchResult struct {
	// Tasks are index for tasks that include matched containers
	Tasks []int
	// Containers are index for matched containers. containers should show up in the original order of the task list and container definitions.
	Containers []MatchedContainer
}

type MatchedContainer struct {
	TaskIndex      int // Index in task list
	ContainerIndex int // Index within a tasks definition's container list
	Targets        []MatchedTarget
}

// MergeTargets adds new targets to the set, the 'key' is port + metrics path.
// The 'key' does not contain an IP address because all targets from one
// container have the same IP address. If there are duplicate 'key's we honor
// the existing target and do not override.  Duplication could happen if there
// are several rules matching same target.
func (mc *MatchedContainer) MergeTargets(newTargets []MatchedTarget) {
NextNewTarget:
	for _, newt := range newTargets {
		for _, old := range mc.Targets {
			// If port and metrics_path are same, then we treat them as same target and keep the existing one
			if old.Port == newt.Port && old.MetricsPath == newt.MetricsPath {
				continue NextNewTarget
			}
		}
		mc.Targets = append(mc.Targets, newt)
	}
}

// MatchedTarget contains info for exporting prometheus scrape target
// and tracing back into the config (can be used in stats, error reporting etc.).
type MatchedTarget struct {
	MatcherType  MatcherType
	MatcherIndex int // Index within a specific matcher type
	Port         int
	MetricsPath  string
	Job          string
}

func newMatchers(c Config, mOpt MatcherOptions) (map[MatcherType][]Matcher, error) {
	// We can have a registry or factory methods etc. but we only have three type of matchers
	// and likely not going to add anymore in forseable future, just hard code the map here.
	// All the XXXConfigToMatchers looks like copy pasted funcs, but there is no generic way to do it.
	matcherConfigs := map[MatcherType][]matcherConfig{
		MatcherTypeService:        serviceConfigsToMatchers(c.Services),
		MatcherTypeTaskDefinition: taskDefinitionConfigsToMatchers(c.TaskDefinitions),
		MatcherTypeDockerLabel:    dockerLabelConfigToMatchers(c.DockerLabels),
	}
	matchers := make(map[MatcherType][]Matcher)
	matcherCount := 0
	for mType, cfgs := range matcherConfigs {
		for i, cfg := range cfgs {
			m, err := cfg.newMatcher(mOpt)
			if err != nil {
				return nil, fmt.Errorf("init matcher config failed type %s index %d: %w", mType, i, err)
			}
			matchers[mType] = append(matchers[mType], m)
			matcherCount++
		}
	}
	if matcherCount == 0 {
		return nil, fmt.Errorf("no matcher specified in config")
	}
	return matchers, nil
}

// a global instance because it's expected and we don't care about why the container didn't match (for now).
// In the future we might add a debug flag for each matcher config and return typed error with more detail
// to help user debug. e.g. type ^ngix-*$ does not match nginx-service.
var errNotMatched = fmt.Errorf("container not matched")

// matchContainers apply one matcher to a list of tasks and returns MatchResult.
// It does not modify the task in place, the attaching match result logic is
// performed by TaskFilter at later stage.
func matchContainers(tasks []*Task, matcher Matcher, matcherIndex int) (*MatchResult, error) {
	var (
		matchedTasks      []int
		matchedContainers []MatchedContainer
	)
	var merr error
	tpe := matcher.Type()
	for tIndex, t := range tasks {
		var matched []MatchedContainer
		for cIndex, c := range t.Definition.ContainerDefinitions {
			targets, err := matcher.MatchTargets(t, c)
			// NOTE: we don't stop when there is an error because it could be one task having invalid docker label.
			if err != nil {
				// Keep track of unexpected error
				if err != errNotMatched {
					multierr.AppendInto(&merr, err)
				}
				continue
			}
			for i := range targets {
				targets[i].MatcherType = tpe
				targets[i].MatcherIndex = matcherIndex
			}
			matched = append(matched, MatchedContainer{
				TaskIndex:      tIndex,
				ContainerIndex: cIndex,
				Targets:        targets,
			})
		}
		if len(matched) > 0 {
			matchedTasks = append(matchedTasks, tIndex)
			matchedContainers = append(matchedContainers, matched...)
		}
	}
	return &MatchResult{
		Tasks:      matchedTasks,
		Containers: matchedContainers,
	}, merr
}

// matchContainerByName is used by taskDefinitionMatcher and serviceMatcher.
// The only exception is DockerLabelMatcher because it get ports from docker label.
func matchContainerByName(nameRegex *regexp.Regexp, expSetting *commonExportSetting, container *ecs.ContainerDefinition) ([]MatchedTarget, error) {
	if nameRegex != nil && !nameRegex.MatchString(aws.StringValue(container.Name)) {
		return nil, errNotMatched
	}
	// Match based on port
	var targets []MatchedTarget
	// Only export container if it has at least one matching port.
	for _, portMapping := range container.PortMappings {
		port := int(aws.Int64Value(portMapping.ContainerPort))
		if expSetting.hasContainerPort(port) {
			targets = append(targets, MatchedTarget{
				Port:        port,
				MetricsPath: expSetting.MetricsPath,
				Job:         expSetting.JobName,
			})
		}
	}
	return targets, nil
}
