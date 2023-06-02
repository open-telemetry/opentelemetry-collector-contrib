// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
