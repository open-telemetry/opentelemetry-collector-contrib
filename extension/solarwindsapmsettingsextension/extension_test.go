package solarwindsapmsettingsextension

import (
	"context"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"testing"
)

func TestCreateExtension(t *testing.T) {
	conf := &Config{
		Endpoint: "apm.collector.cloud.solarwinds.com:443",
		Key:      "vlEW1JtimSH2LlBsNrPdeEjBxNl5z8Bp7gX55bNTk3_GIxHWedgj42GDFBWpRe2ne7TffHk:jerry_test",
		Interval: "1s",
	}
	ex := createAnExtension(conf, t)
	ex.Shutdown(context.TODO())
}

func TestCreateExtensionWrongEndpoint(t *testing.T) {
	conf := &Config{
		Endpoint: "apm.collector.cloud.solarwindsswoswoswo.com:443",
		Key:      "vlEW1JtimSH2LlBsNrPdeEjBxNl5z8Bp7gX55bNTk3_GIxHWedgj42GDFBWpRe2ne7TffHk:jerry_test",
		Interval: "1s",
	}
	ex := createAnExtension(conf, t)
	ex.Shutdown(context.TODO())
}

func TestCreateExtensionWrongKey(t *testing.T) {
	conf := &Config{
		Endpoint: "apm.collector.cloud.solarwinds.com:443",
		Key:      "IsItAKey:jerry_test",
		Interval: "1s",
	}
	ex := createAnExtension(conf, t)
	ex.Shutdown(context.TODO())
}

// create extension
func createAnExtension(c *Config, t *testing.T) extension.Extension {
	logger, err := zap.NewProduction()
	ex, err := newSolarwindsApmSettingsExtension(c, logger)
	require.NoError(t, err)
	err = ex.Start(context.TODO(), nil)
	require.NoError(t, err)
	return ex
}
