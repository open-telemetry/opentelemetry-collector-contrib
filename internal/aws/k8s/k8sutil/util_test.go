// Copyright The OpenTelemetry Authors
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

package k8sutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatePodKey(t *testing.T) {
	assert.Equal(t, "namespace:default,podName:testPod", CreatePodKey("default", "testPod"))
	assert.Equal(t, "", CreatePodKey("", "testPod"))
	assert.Equal(t, "", CreatePodKey("default", ""))
	assert.Equal(t, "", CreatePodKey("", ""))
}

func TestCreateContainerKey(t *testing.T) {
	assert.Equal(t, "namespace:default,podName:testPod,containerName:testContainer", CreateContainerKey("default", "testPod", "testContainer"))
	assert.Equal(t, "", CreateContainerKey("", "testPod", "testContainer"))
	assert.Equal(t, "", CreateContainerKey("default", "", "testContainer"))
	assert.Equal(t, "", CreateContainerKey("default", "testPod", ""))
}
