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

package ecsinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetContainerInstanceIdFromArn(t *testing.T) {

	oldFormatARN := "arn:aws:ecs:region:aws_account_id:task/task-id"
	newFormatARN := "arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id"

	result, _ := GetContainerInstanceIdFromArn(oldFormatARN)
	assert.Equal(t, "task-id", result, "Expected to be equal")
	result, _ = GetContainerInstanceIdFromArn(newFormatARN)
	assert.Equal(t, "task-id", result, "Expected to be equal")
	wrongFormatARN := "arn:aws:ecs:region:aws_account_id:task"
	result, err := GetContainerInstanceIdFromArn(wrongFormatARN)
	assert.NotNil(t, err)
}
