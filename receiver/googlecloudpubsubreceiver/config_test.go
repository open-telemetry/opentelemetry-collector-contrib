// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: &Config{},
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				ProjectID: "my-project",
				UserAgent: "opentelemetry-collector-contrib {{version}}",
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 20 * time.Second,
				},
				Subscription: "projects/my-project/subscriptions/otlp-subscription",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	assert.Error(t, c.validateForTrace())
	assert.Error(t, c.validateForLog())
	assert.Error(t, c.validateForMetric())
	c.Subscription = "projects/000project/subscriptions/my-subscription"
	assert.Error(t, c.validate())
	c.Subscription = "projects/my-project/topics/my-topic"
	assert.Error(t, c.validate())
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validate())
}

func TestTraceConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validateForTrace())

	c.Encoding = "otlp_proto_metric"
	assert.Error(t, c.validateForTrace())
	c.Encoding = "otlp_proto_log"
	assert.Error(t, c.validateForTrace())
	c.Encoding = "raw_text"
	assert.Error(t, c.validateForTrace())
	c.Encoding = "raw_json"
	assert.Error(t, c.validateForTrace())

	c.Encoding = "otlp_proto_trace"
	assert.NoError(t, c.validateForTrace())
}

func TestMetricConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validateForMetric())

	c.Encoding = "otlp_proto_trace"
	assert.Error(t, c.validateForMetric())
	c.Encoding = "otlp_proto_log"
	assert.Error(t, c.validateForMetric())
	c.Encoding = "raw_text"
	assert.Error(t, c.validateForMetric())
	c.Encoding = "raw_json"
	assert.Error(t, c.validateForMetric())

	c.Encoding = "otlp_proto_metric"
	assert.NoError(t, c.validateForMetric())
}

func TestLogConfigValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Subscription = "projects/my-project/subscriptions/my-subscription"
	assert.NoError(t, c.validateForLog())

	c.Encoding = "otlp_proto_trace"
	assert.Error(t, c.validateForLog())
	c.Encoding = "otlp_proto_metric"
	assert.Error(t, c.validateForLog())

	c.Encoding = "raw_text"
	assert.NoError(t, c.validateForLog())
	c.Encoding = "raw_json"
	assert.NoError(t, c.validateForLog())
	c.Encoding = "otlp_proto_log"
	assert.NoError(t, c.validateForLog())
}

func TestConfigCredentialsValidation(t *testing.T) {
	factory := NewFactory()
	c := factory.CreateDefaultConfig().(*Config)
	c.Subscription = "projects/my-project/subscriptions/my-subscription"

	dummyCredentialsJSON := `{
		"type": "service_account",
		"project_id": "my-project",
		"private_key_id": "d41d8cd98f00b204e9800998ecf8427e",
		"private_key": "-----BEGIN PRIVATE KEY-----\njciFXV3SnwCNXBLSykSNBQ\n-----END PRIVATE KEY-----\n",
		"client_email": "me@my-project.iam.gserviceaccount.com",
		"client_id": "87689569482041111",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-project.iam.gserviceaccount.com",
		"universe_domain": "googleapis.com"
	  }`

	c.CredentialsFilePath = "/home/service-principal-01ba5c7dde9b.json"
	c.CredentialsJSON = ""
	assert.NoError(t, c.validate())

	c.CredentialsFilePath = ""
	c.CredentialsJSON = dummyCredentialsJSON
	assert.NoError(t, c.validate())

	c.CredentialsFilePath = "/home/service-principal-01ba5c7dde9b.json"
	c.CredentialsJSON = dummyCredentialsJSON
	assert.Error(t, c.validate(), "only one of")

	c.CredentialsFilePath = ""
	c.CredentialsJSON = "invalid JSON"
	assert.Error(t, c.validate(), "credentials_json is an invalid JSON")
}
