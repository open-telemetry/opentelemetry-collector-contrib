package gitmetricsreceiver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal"
)

var creationSet = receivertest.NewNopCreateSettings()

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mReceiver)
}

func TestCreateReceiver_ScraperKeyConfigError(t *testing.T) {
	const errorKey string = "error"

	factory := NewFactory()
	cfg := &Config{Scrapers: map[string]internal.Config{errorKey: &mockConfig{}}}

	_, err := factory.CreateMetricsReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.EqualError(t, err, fmt.Sprintf("git metrics scraper factory not found for key: %q", errorKey))
}

