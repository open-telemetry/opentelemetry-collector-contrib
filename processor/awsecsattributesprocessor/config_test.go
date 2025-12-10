package awsecsattributesprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	_, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	require.NotNil(t, factory)

	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				CacheTTL: 60,
				Attributes: []string{
					"^aws.*",
					"^docker.*",
				},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with multiple sources",
			config: &Config{
				CacheTTL: 300,
				Attributes: []string{
					".*",
				},
				ContainerID: ContainerID{
					Sources: []string{"container.id", "log.file.name"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - no container ID sources",
			config: &Config{
				CacheTTL:   60,
				Attributes: []string{"^aws.*"},
				ContainerID: ContainerID{
					Sources: []string{},
				},
			},
			wantErr: true,
			errMsg:  "atleast one container ID source must be specified",
		},
		{
			name: "invalid - cache TTL too low",
			config: &Config{
				CacheTTL:   30,
				Attributes: []string{"^aws.*"},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},
			wantErr: true,
			errMsg:  "cache_ttl cannot be less than 60 seconds",
		},
		{
			name: "invalid - bad regex pattern",
			config: &Config{
				CacheTTL: 60,
				Attributes: []string{
					"?=",
				},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},
			wantErr: true,
			errMsg:  "invalid expression found under attributes pattern",
		},
		{
			name: "valid - empty attributes list",
			config: &Config{
				CacheTTL:   60,
				Attributes: []string{},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid - minimum cache TTL",
			config: &Config{
				CacheTTL:   60,
				Attributes: []string{"^aws.*"},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfigInit(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "successful init with valid regex",
			config: &Config{
				CacheTTL: 60,
				Attributes: []string{
					"^aws.*",
					"^docker.*",
				},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},
			wantErr: false,
		},
		{
			name: "init fails with invalid regex",
			config: &Config{
				CacheTTL: 60,
				Attributes: []string{
					"?=",
				},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},
			wantErr: true,
		},
		{
			name: "successful init with empty attributes",
			config: &Config{
				CacheTTL:   60,
				Attributes: []string{},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.init()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// Verify regex expressions were compiled
				if len(tt.config.Attributes) > 0 {
					require.Equal(t, len(tt.config.Attributes), len(tt.config.attrExpressions))
				}
			}
		})
	}
}

func TestConfigAllowAttr(t *testing.T) {
	config := &Config{
		Attributes: []string{
			"^aws.*",
			"^docker.*",
			"^image.*",
		},
		CacheTTL: 60,
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}

	// Initialize to compile regex patterns
	err := config.init()
	require.NoError(t, err)

	tests := []struct {
		name     string
		attrKey  string
		expected bool
	}{
		{
			name:     "matches aws pattern",
			attrKey:  "aws.ecs.cluster",
			expected: true,
		},
		{
			name:     "matches docker pattern",
			attrKey:  "docker.id",
			expected: true,
		},
		{
			name:     "matches image pattern",
			attrKey:  "image.id",
			expected: true,
		},
		{
			name:     "does not match any pattern",
			attrKey:  "random.attribute",
			expected: false,
		},
		{
			name:     "partial match should work",
			attrKey:  "aws.ecs.task.arn",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.allowAttr(tt.attrKey)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigAllowAttr_EmptyPatterns(t *testing.T) {
	config := &Config{
		Attributes: []string{},
		CacheTTL:   60,
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}

	err := config.init()
	require.NoError(t, err)

	// When no patterns are specified, all attributes should be allowed
	require.True(t, config.allowAttr("any.attribute"))
	require.True(t, config.allowAttr("another.attribute"))
}
