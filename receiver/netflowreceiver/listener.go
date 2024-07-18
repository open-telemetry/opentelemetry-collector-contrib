// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"github.com/netsampler/goflow2/v2/utils"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type Listener struct {
	config      ListenerConfig
	logger      *zap.Logger
	recv        *utils.UDPReceiver
	logConsumer consumer.Logs
}
