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
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// error.go defines common error interfaces and util methods for generating reports
// for log and metrics that can be used for debugging.

type errWithAttributes interface {
	// message does not include attributes like task arn etc.
	// and expect the caller extract them using getters.
	message() string
	// zapFields will be logged as json attribute and allows searching and filter backend like cloudwatch.
	// For example { $.ErrScope == "Target" } list all the error whose scope is a (scrape) target.
	zapFields() []zap.Field
}

type taskError interface {
	errWithAttributes
	setTask(t *Task)
	getTask() *Task
}

// baseTaskError can be embedded into error that happens at task level or below.
type baseTaskError struct {
	t *Task
}

func (be *baseTaskError) setTask(t *Task) {
	be.t = t
}

func (be *baseTaskError) getTask() *Task {
	return be.t
}

func setErrTask(err error, t *Task) {
	e, ok := err.(taskError)
	if !ok {
		return
	}
	e.setTask(t)
}

type containerError interface {
	taskError
	setContainerIndex(j int)
	getContainerIndex() int
}

func setErrContainer(err error, containerIndex int, task *Task) {
	e, ok := err.(containerError)
	if !ok {
		return
	}
	setErrTask(err, task)
	e.setContainerIndex(containerIndex)
}

type targetError interface {
	containerError
	setTarget(t MatchedTarget)
	getTarget() MatchedTarget
}

func setErrTarget(err error, target MatchedTarget, containerIndex int, task *Task) {
	e, ok := err.(targetError)
	if !ok {
		return
	}
	setErrContainer(err, containerIndex, task)
	e.setTarget(target)
}

func printErrors(logger *zap.Logger, err error) {
	merr := multierr.Errors(err)
	if merr == nil {
		return
	}

	for _, err := range merr {
		m := err.Error()
		serr, ok := err.(errWithAttributes)
		if ok {
			m = serr.message()
		}
		fields, scope := extractErrorFields(err)
		fields = append(fields, zap.String("ErrScope", scope))
		logger.Error(m, fields...)
	}
}

func extractErrorFields(err error) ([]zap.Field, string) {
	var fields []zap.Field
	scope := "Unknown"
	errAttr, ok := err.(errWithAttributes)
	if !ok {
		return fields, scope
	}
	fields = errAttr.zapFields()

	taskErr, ok := err.(taskError)
	if !ok {
		return fields, scope
	}
	scope = "Task"
	task := taskErr.getTask()
	fields = append(fields, zap.String("TaskArn", aws.StringValue(task.Task.TaskArn)))

	containerErr, ok := err.(containerError)
	if !ok {
		return fields, scope
	}
	scope = "Container"
	container := task.Task.Containers[containerErr.getContainerIndex()]
	fields = append(fields, zap.String("ContainerName", aws.StringValue(container.Name)))
	if !ok {
		return fields, scope
	}

	targetErr, ok := err.(targetError)
	if !ok {
		return fields, scope
	}
	scope = "Target"
	target := targetErr.getTarget()
	// TODO: change to string once another PR for matcher got merged
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/3386 defines Stringer
	fields = append(fields, zap.Int("MatcherType", int(target.MatcherType)))
	return fields, scope
}
