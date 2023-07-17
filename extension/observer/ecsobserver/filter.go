// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"sort"

	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type taskFilter struct {
	logger   *zap.Logger
	matchers map[matcherType][]targetMatcher
}

func newTaskFilter(logger *zap.Logger, matchers map[matcherType][]targetMatcher) *taskFilter {
	return &taskFilter{
		logger:   logger,
		matchers: matchers,
	}
}

// Filter run all the matchers and return all the tasks that including at least one matched container.
func (f *taskFilter) filter(tasks []*taskAnnotated) ([]*taskAnnotated, error) {
	// Group result by matcher type, each type can have multiple configs.
	matched := make(map[matcherType][]*matchResult)
	var merr error
	for tpe, matchers := range f.matchers { // for each type of matchers
		for index, matcher := range matchers { // for individual matchers of same type
			res, err := matchContainers(tasks, matcher, index)
			// NOTE: we continue the loop even if there is error because some tasks can has invalid labels.
			// matchContainers always return non nil result even if there are errors during matching.
			if err != nil {
				multierr.AppendInto(&merr, err)
			}

			// TODO: print out the pattern to include both pattern and port
			f.logger.Debug("matched",
				zap.String("matcherType", tpe.String()), zap.Int("MatcherIndex", index),
				zap.Int("Tasks", len(tasks)), zap.Int("MatchedTasks", len(res.Tasks)),
				zap.Int("MatchedContainers", len(res.Containers)))
			matched[tpe] = append(matched[tpe], res)
		}
	}

	// Attach match result to tasks, do it in matcherOrders.
	// AddMatchedContainer will merge in new targets if it is not already matched by other matchers.
	matchedTasks := make(map[int]struct{})
	for _, tpe := range matcherOrders() {
		for _, res := range matched[tpe] {
			for _, container := range res.Containers {
				matchedTasks[container.TaskIndex] = struct{}{}
				task := tasks[container.TaskIndex]
				task.AddMatchedContainer(container)
			}
		}
	}

	// Sort by task index so the output is consistent.
	var taskIndexes []int
	for k := range matchedTasks {
		taskIndexes = append(taskIndexes, k)
	}
	sort.Ints(taskIndexes)
	var sortedTasks []*taskAnnotated
	for _, i := range taskIndexes {
		task := tasks[i]
		// Sort containers within a task
		sort.Slice(task.Matched, func(i, j int) bool {
			return task.Matched[i].ContainerIndex < task.Matched[j].ContainerIndex
		})
		sortedTasks = append(sortedTasks, task)
	}
	return sortedTasks, merr
}
