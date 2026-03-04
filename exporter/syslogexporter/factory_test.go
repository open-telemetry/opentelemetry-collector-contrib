// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter

import (
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, metadata.Type)
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()

	assert.Equal(t, &Config{
		Port:     514,
		Network:  "tcp",
		Protocol: "rfc5424",
		QueueSettings: func() exporterhelper.QueueBatchConfig {
			queue := exporterhelper.NewDefaultQueueConfig()
			queue.Enabled = false
			queue.NumConsumers = 10
			queue.QueueSize = 1000
			queue.Sizer = exporterhelper.RequestSizerTypeRequests
			return queue
		}(),
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5 * time.Second,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
			MaxInterval:         30 * time.Second,
			MaxElapsedTime:      5 * time.Minute,
		},
		TimeoutSettings: exporterhelper.TimeoutConfig{
			Timeout: 5 * time.Second,
		},
	}, cfg)
}
