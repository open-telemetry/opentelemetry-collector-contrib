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

func TestParseInstanceIdFromProviderId(t *testing.T) {
	assert.Equal(t, "i-0b00e07ccd388f915", ParseInstanceIdFromProviderId("aws:///us-west-2b/i-0b00e07ccd388f915"))
	assert.Equal(t, "i-0b00e07ccd388f915", ParseInstanceIdFromProviderId("aws:///us-east-1c/i-0b00e07ccd388f915"))
	assert.Equal(t, "", ParseInstanceIdFromProviderId(":///us-east-1c/i-0b00e07ccd388f915"))
	assert.Equal(t, "", ParseInstanceIdFromProviderId(""))
}
