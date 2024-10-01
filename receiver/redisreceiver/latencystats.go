// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"fmt"
	"strconv"
	"strings"
)

// Holds percentile latencies, e.g. "p99" -> 1.5.
type latencies map[string]float64

// parseLatencyStats parses the values part of one entry in Redis latencystats section,
// e.g. "p50=181.247,p99=309.247,p99.9=1023.999".
func parseLatencyStats(str string) (latencies, error) {
	res := make(latencies)

	pairs := strings.Split(strings.TrimSpace(str), ",")

	for _, pairStr := range pairs {
		pair := strings.Split(pairStr, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf("unexpected latency percentiles pair '%s'", pairStr)
		}

		key := pair[0]
		valueStr := pair[1]

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return nil, err
		}

		res[key] = value
	}

	return res, nil
}
