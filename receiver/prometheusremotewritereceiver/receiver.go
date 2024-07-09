// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"context"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"net/http"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// NewReceiver - remote write
func NewReceiver(settings receiver.Settings, cfg *Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	panic("need implement")
}

type PrometheusRemoteWriteReceiver struct {
	settings     receiver.Settings
	host         component.Host
	nextConsumer consumer.Metrics

	shutdownWG sync.WaitGroup

	server  *http.Server
	config  *Config
	logger  *zap.Logger
	obsrecv *receiverhelper.ObsReport
}

func (prwc *PrometheusRemoteWriteReceiver) Start(ctx context.Context, host component.Host) error {
	panic("need implement")
}
