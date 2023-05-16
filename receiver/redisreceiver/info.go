// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
