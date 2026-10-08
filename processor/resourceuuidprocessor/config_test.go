package resourceuuidprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfigIsValid(t *testing.T) {
	require.NoError(t, createDefaultConfig().(*Config).Validate())
}

func TestConfigValidate(t *testing.T) {
	mutations := map[string]func(*Config){
		"empty endpoint":         func(c *Config) { c.Endpoint = "" },
		"zero refresh":           func(c *Config) { c.RefreshInterval = 0 },
		"zero retry":             func(c *Config) { c.RetryInterval = 0 },
		"zero ttl":               func(c *Config) { c.CacheTTL = 0 },
		"zero timeout":           func(c *Config) { c.Timeout = 0 },
		"zero cache size":        func(c *Config) { c.CacheSize = 0 },
		"retry greater than ttl": func(c *Config) { c.RetryInterval = time.Hour },
		"empty pod uid attr":     func(c *Config) { c.PodUIDAttribute = "" },
		"empty target attr":      func(c *Config) { c.TargetAttribute = "" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			mutate(cfg)
			require.Error(t, cfg.Validate())
		})
	}
}
