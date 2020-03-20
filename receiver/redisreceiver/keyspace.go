// Copyright 2020, OpenTelemetry Authors
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

package redisreceiver

import (
	"strconv"
	"strings"
)

// Holds fields returned by the Keyspace section of the INFO command: e.g.
// "db0:keys=1,expires=2,avg_ttl=3"
type keyspace struct {
	db      string
	keys    int
	expires int
	avgTtl  int
}

// Turns a keyspace value (the part after the colon
// e.g. "keys=1,expires=2,avg_ttl=3") into a keyspace struct
func parseKeyspaceString(db int, str string) (*keyspace, error) {
	pairs := strings.Split(str, ",")
	out := keyspace{db: strconv.Itoa(db)}
	for _, pairStr := range pairs {
		pair := strings.Split(pairStr, "=")
		key := pair[0]
		val, err := strconv.Atoi(pair[1])
		if err != nil {
			return nil, err
		}
		switch key {
		case "keys":
			out.keys = val
		case "expires":
			out.expires = val
		case "avg_ttl":
			out.avgTtl = val
		}
	}
	return &out, nil
}
