// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"fmt"
	"strconv"
	"strings"
)

// Holds fields returned by the Keyspace section of the INFO command: e.g.
// "db0:keys=1,expires=2,avg_ttl=3"
type keyspace struct {
	db      string
	keys    int
	expires int
	avgTTL  int
}

// Turns a keyspace value (the part after the colon
// e.g. "keys=1,expires=2,avg_ttl=3") into a keyspace struct
func parseKeyspaceString(db int, str string) (*keyspace, error) {
	pairs := strings.Split(str, ",")
	ks := keyspace{db: strconv.Itoa(db)}
	for _, pairStr := range pairs {
		var field *int
		pair := strings.Split(pairStr, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf(
				"unexpected keyspace pair '%s'",
				pairStr,
			)
		}
		key := pair[0]
		switch key {
		case "keys":
			field = &ks.keys
		case "expires":
			field = &ks.expires
		case "avg_ttl":
			field = &ks.avgTTL
		}
		if field != nil {
			val, err := strconv.Atoi(pair[1])
			if err != nil {
				return nil, err
			}
			*field = val
		}
	}
	return &ks, nil
}
