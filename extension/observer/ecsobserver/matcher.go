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
	"github.com/aws/aws-sdk-go/service/ecs"
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

type MatchedTarget struct {
	MatcherType  MatcherType
	MatcherIndex int // Index within a specific matcher type
	Port         int
	MetricsPath  string
	Job          string
}
