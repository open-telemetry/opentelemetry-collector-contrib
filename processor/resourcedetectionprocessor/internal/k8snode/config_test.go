// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetCustomEnvVar(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.NodeFromEnvVar = "my_env_var"
	t.Setenv("my_env_var", "node1")
	err := cfg.UpdateDefaults()
	assert.NoError(t, err)
}

func TestSetCustomEnvVarEmptyVal(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.NodeFromEnvVar = "my_env_var"
	err := cfg.UpdateDefaults()
	assert.Error(t, err)
	assert.EqualError(t, err, "node name can't be found. Check the readme on how to set the required env variable")
}

func TestSetDefaultEnvVar(t *testing.T) {
	cfg := CreateDefaultConfig()
	t.Setenv("K8S_NODE_NAME", "node1")
	err := cfg.UpdateDefaults()
	assert.NoError(t, err)
}
