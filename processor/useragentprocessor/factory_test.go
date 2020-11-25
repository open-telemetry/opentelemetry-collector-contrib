package useragentprocessor

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), configmodels.Type(typeStr))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			NameVal: typeStr,
			TypeVal: typeStr,
		},
		UserAgentTag:      "userAgent",
		UserAgentFilePath: "processor/useragentprocessor/resources/regexes.yaml",
	})
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestFactoryCreateTraceProcessor_InvalidFilePath(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.UserAgentFilePath = path.Join("tmp", "regexes.yaml")
	ap, err := factory.CreateTracesProcessor(context.Background(), component.ProcessorCreateParams{}, cfg, consumertest.NewTracesNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateTraceProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.UserAgentFilePath = path.Join(".", "resources", "regexes.yaml")
	tp, err := factory.CreateTracesProcessor(context.Background(), component.ProcessorCreateParams{}, cfg, consumertest.NewTracesNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)
}
