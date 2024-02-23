// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Endpoint string `mapstructure:"endpoint"`
	Key      string `mapstructure:"key"`
	Interval string `mapstructure:"interval"`
}

func (cfg *Config) Validate() error {
	if len(cfg.Endpoint) == 0 {
		return errors.New("endpoint must not be empty")
	}
	endpointArr := strings.Split(cfg.Endpoint, ":")
	if len(endpointArr) != 2 {
		return errors.New("endpoint should be in \"<host>:<port>\" format")
	}
	if _, err := strconv.Atoi(endpointArr[1]); err != nil {
		return errors.New("the <port> portion of endpoint has to be an integer")
	}
	if len(cfg.Key) == 0 {
		return errors.New("key must not be empty")
	}
	keyArr := strings.Split(cfg.Key, ":")
	if len(keyArr) != 2 {
		return errors.New("key should be in \"<token>:<service_name>\" format")
	}
	if _, err := time.ParseDuration(cfg.Interval); err != nil {
		return errors.New("interval has to be a duration string. Valid time units are \"ns\", \"us\" (or \"µs\"), \"ms\", \"s\", \"m\", \"h\"")
	}
	return nil
}
