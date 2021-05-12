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

type MatcherConfig interface {
	// Init validates the configuration and initializes some internal strcutrues like regexp.
	Init() error
	NewMatcher(options MatcherOptions) (Matcher, error)
}

type MatcherOptions struct {
	Logger *zap.Logger
}

type MatcherType int

const (
	MatcherTypeService MatcherType = iota + 1
	MatcherTypeTaskDefinition
	MatcherTypeDockerLabel
)

type MatchResult struct {
	// Tasks are index for tasks that include matched containers
	Tasks []int
	// Containers are index for matched containers. containers should show up in the original order of the task list and container definitions.
	Containers []MatchedContainer
}

type MatchedContainer struct {
	TaskIndex      int // Index in task list
	ContainerIndex int // Index within a tasks defintion's container list
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
