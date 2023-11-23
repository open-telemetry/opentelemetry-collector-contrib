package rabbitmqexporter

import "time"

type config struct {
	connectionUrl               string
	channelPoolSize             int
	connectionTimeout           time.Duration
	connectionHeartbeatInterval time.Duration
	confirmMode                 bool
}
