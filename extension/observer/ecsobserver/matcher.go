// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type targetMatcher interface {
	// type() won't parse
	matcherType() matcherType
	// matchTargets returns targets fond from the specific container.
	// One container can have multiple targets because it may have multiple ports.
	matchTargets(task *taskAnnotated, container ecstypes.ContainerDefinition) ([]matchedTarget, error)
}

// matcherConfig should be implemented by all the matcher config structs
// for validation and initializing the actual matcher implementation.
type matcherConfig interface {
	// validate calls NewMatcher and only returns the error, it can be used in test
	// and the new config validator interface.
	validate() error
	// newMatcher validates config and creates a targetMatcher implementation.
	// The error is a config validation error
	newMatcher(options matcherOptions) (targetMatcher, error)
}

type matcherOptions struct {
	Logger *zap.Logger
}

type matcherType int

// Values for enum matcherType.
const (
	matcherTypeService matcherType = iota + 1
	matcherTypeTaskDefinition
	matcherTypeDockerLabel
)

func (t matcherType) String() string {
	switch t {
	case matcherTypeService:
		return "service"
	case matcherTypeTaskDefinition:
		return "task_definition"
	case matcherTypeDockerLabel:
		return "docker_label"
	default:
		// Give it a _matcher_type suffix so people can find it by string search.
		return "unknown_matcher_type"
	}
}

type matchResult struct {
	// Tasks are index for tasks that include matched containers
	Tasks []int
	// Containers are index for matched containers. containers should show up in the original order of the task list and container definitions.
	Containers []matchedContainer
}

type matchedContainer struct {
	TaskIndex      int // Index in task list before filter, i.e. after fetch and decorate
	ContainerIndex int // Index within a tasks definition's container list
	Targets        []matchedTarget
}

// MergeTargets adds new targets to the set, the 'key' is port + metrics path.
// The 'key' does not contain an IP address because all targets from one
// container have the same IP address. If there are duplicate 'key's we honor
// the existing target and do not override.  Duplication could happen if there
// are several rules matching same target.
func (mc *matchedContainer) MergeTargets(newTargets []matchedTarget) {
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

// matchedTarget contains info for exporting prometheus scrape target
// and tracing back into the config (can be used in stats, error reporting etc.).
type matchedTarget struct {
	MatcherType  matcherType
	MatcherIndex int // Index within a specific matcher type
	Port         int
	MetricsPath  string
	Job          string
}

func matcherOrders() []matcherType {
	return []matcherType{
		matcherTypeService,
		matcherTypeTaskDefinition,
		matcherTypeDockerLabel,
	}
}

func newMatchers(c Config, mOpt matcherOptions) (map[matcherType][]targetMatcher, error) {
	// We can have a registry or factory methods etc. but we only have three type of matchers
	// and likely not going to add anymore in forseable future, just hard code the map here.
	// All the XXXConfigToMatchers looks like copy pasted funcs, but there is no generic way to do it.
	matcherConfigs := map[matcherType][]matcherConfig{
		matcherTypeService:        serviceConfigsToMatchers(c.Services),
		matcherTypeTaskDefinition: taskDefinitionConfigsToMatchers(c.TaskDefinitions),
		matcherTypeDockerLabel:    dockerLabelConfigToMatchers(c.DockerLabels),
	}
	matchers := make(map[matcherType][]targetMatcher)
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
		return nil, errors.New("no matcher specified in config")
	}
	return matchers, nil
}

// a global instance because it's expected and we don't care about why the container didn't match (for now).
// In the future we might add a debug flag for each matcher config and return typed error with more detail
// to help user debug. e.g. type ^ngix-*$ does not match nginx-service.
var errNotMatched = errors.New("container not matched")

// matchContainers apply one matcher to a list of tasks and returns matchResult.
// It does not modify the task in place, the attaching match result logic is
// performed by taskFilter at later stage.
func matchContainers(tasks []*taskAnnotated, matcher targetMatcher, matcherIndex int) (*matchResult, error) {
	var (
		matchedTasks      []int
		matchedContainers []matchedContainer
	)
	var merr error
	tpe := matcher.matcherType()
	for tIndex, t := range tasks {
		var matched []matchedContainer
		for cIndex, c := range t.Definition.ContainerDefinitions {
			targets, err := matcher.matchTargets(t, c)
			// NOTE: we don't stop when there is an error because it could be one task having invalid docker label.
			if err != nil {
				// Keep track of unexpected error
				if !errors.Is(err, errNotMatched) {
					multierr.AppendInto(&merr, err)
				}
				continue
			}
			for i := range targets {
				targets[i].MatcherType = tpe
				targets[i].MatcherIndex = matcherIndex
			}
			matched = append(matched, matchedContainer{
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
	return &matchResult{
		Tasks:      matchedTasks,
		Containers: matchedContainers,
	}, merr
}

// matchContainerByName is used by taskDefinitionMatcher and serviceMatcher.
// The only exception is DockerLabelMatcher because it get ports from docker label.
func matchContainerByName(nameRegex *regexp.Regexp, expSetting *commonExportSetting, container ecstypes.ContainerDefinition) ([]matchedTarget, error) {
	if nameRegex != nil && !nameRegex.MatchString(aws.ToString(container.Name)) {
		return nil, errNotMatched
	}
	// Match based on port
	var targets []matchedTarget
	// Only export container if it has at least one matching port.
	for _, portMapping := range container.PortMappings {
		port := int(aws.ToInt32(portMapping.ContainerPort))
		if expSetting.hasContainerPort(port) {
			targets = append(targets, matchedTarget{
				Port:        port,
				MetricsPath: expSetting.MetricsPath,
				Job:         expSetting.JobName,
			})
		}
	}
	return targets, nil
}
