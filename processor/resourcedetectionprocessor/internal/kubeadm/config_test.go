package kubeadm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetCustomEnvVar(t *testing.T) {
	cfg := CreateDefaultConfig()
	err := cfg.UpdateDefaults()
	assert.NoError(t, err)
	assert.Equal(t, defaultConfigMapName, cfg.configMapName)
	assert.Equal(t, defaultConfigMapNamespace, cfg.configMapNamespace)
}
