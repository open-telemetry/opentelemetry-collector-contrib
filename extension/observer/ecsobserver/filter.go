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
	"sort"

	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type TaskFilter struct {
	logger   *zap.Logger
	matchers map[MatcherType][]Matcher
}

func NewTaskFilter(c Config, logger *zap.Logger, matchers map[MatcherType][]Matcher) (*TaskFilter, error) {
	return &TaskFilter{
		logger:   logger,
		matchers: matchers,
	}, nil
}

// Filter run all the matchers and return all the tasks that including at least one matched container.
// It updates those matched tasks in place. (i.e. it does not do shallow copy).
func (f *TaskFilter) Filter(tasks []*Task) ([]*Task, error) {
	// Gather results from all the matchers
	matched := make(map[MatcherType][]*MatchResult)
	var merr error
	for tpe, matchers := range f.matchers {
		for index, matcher := range matchers {
			res, err := matchContainers(tasks, matcher, index)
			// NOTE: we continue the loop even if there is error because it could some tasks has invalid labels.
			// matchCotnainers always return non nil result even if there are errors during matching.
			if err != nil {
				multierr.AppendInto(&merr, fmt.Errorf("matcher failed with type %s index %d: %w", tpe, index, err))
			}

			f.logger.Debug("matched",
				zap.String("MatcherType", tpe.String()), zap.Int("MatcherIndex", index),
				zap.Int("Tasks", len(tasks)), zap.Int("MatchedTasks", len(res.Tasks)),
				zap.Int("MatchedContainers", len(res.Containers)))
			matched[tpe] = append(matched[tpe], res)
		}
	}

	// Dedupe, key is task index
	matchedTasks := make(map[int]struct{})
	for _, tpe := range matcherOrders() {
		for _, res := range matched[tpe] {
			for _, container := range res.Containers {
				matchedTasks[container.TaskIndex] = struct{}{}
				task := tasks[container.TaskIndex]
				// Merge mached containers into the task, this is in place update.
				// Each task can contain match result from different matchers on different containers.
				// Same container can also contain multiple targets。
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
	var sortedTasks []*Task
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
