package netflowreceiver

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
