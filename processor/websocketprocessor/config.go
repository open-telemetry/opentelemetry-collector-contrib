// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package websocketprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/websocketprocessor"

import (
	"go.opentelemetry.io/collector/component"
	"golang.org/x/time/rate"
)

const defaultPort = 12001

type Config struct {
	// Port indicates the port used by the web socket listener started by this processor.
	// Defaults to 12001.
	Port int `mapstructure:"port"`
	// Limit is a float that indicates the maximum number of messages repeated
	// through the websocket by this processor in messages per second. Defaults to 1.
	Limit rate.Limit `mapstructure:"limit"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Port:  defaultPort,
		Limit: 1,
	}
}
