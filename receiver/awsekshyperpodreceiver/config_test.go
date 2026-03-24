// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsekshyperpodreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		ClusterName:      "my-cluster",
	}
	cfg.CollectionInterval = 60 * time.Second

	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_ZeroInterval(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
	}
	cfg.CollectionInterval = 0

	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_NegativeInterval(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
	}
	cfg.CollectionInterval = -1 * time.Second

	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_SmallPositiveInterval(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
	}
	cfg.CollectionInterval = 1 * time.Nanosecond

	require.NoError(t, cfg.Validate())
}
