// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"errors"
	"strconv"
	"time"
)

// A map of the INFO data returned from Redis.
type info map[string]string

func (i info) getUptimeInSeconds() (time.Duration, error) {
	const uptimeKey = "uptime_in_seconds"
	uptimeStr, ok := i[uptimeKey]
	if !ok {
		return 0, errors.New(uptimeKey + " missing from redis info")
	}
	sec, err := strconv.Atoi(uptimeStr)
	if err != nil {
		return time.Duration(0), err
	}
	return time.Duration(sec) * time.Second, nil
}
