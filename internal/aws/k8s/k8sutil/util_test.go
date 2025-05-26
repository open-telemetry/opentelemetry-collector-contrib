// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatePodKey(t *testing.T) {
	assert.Equal(t, "namespace:default,podName:testPod", CreatePodKey("default", "testPod"))
	assert.Empty(t, CreatePodKey("", "testPod"))
	assert.Empty(t, CreatePodKey("default", ""))
	assert.Empty(t, CreatePodKey("", ""))
}

func TestCreateContainerKey(t *testing.T) {
	assert.Equal(t, "namespace:default,podName:testPod,containerName:testContainer", CreateContainerKey("default", "testPod", "testContainer"))
	assert.Empty(t, CreateContainerKey("", "testPod", "testContainer"))
	assert.Empty(t, CreateContainerKey("default", "", "testContainer"))
	assert.Empty(t, CreateContainerKey("default", "testPod", ""))
}
