package redisreceiver

import (
	"strconv"
	"strings"
)

// Holds fields returned by INFO command: e.g.
// db0:keys=1,expires=2,avg_ttl=3
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
