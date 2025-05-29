// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"time"
)

// NewClickhouseClient creates a new ClickHouse client from a DSN URL string.
func NewClickhouseClient(dsn string) (driver.Conn, error) {
	opt, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	// TODO: put enable_json_type=1 in the DSN
	//opt.Settings["enable_json_type"] = "1"

	conn, err := clickhouse.Open(opt)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// GenerateTTLExpr generates a TTL expression for a ClickHouse table.
func GenerateTTLExpr(ttl time.Duration, timeField string) string {
	if ttl > 0 {
		switch {
		case ttl%(24*time.Hour) == 0:
			return fmt.Sprintf(`TTL %s + toIntervalDay(%d)`, timeField, ttl/(24*time.Hour))
		case ttl%(time.Hour) == 0:
			return fmt.Sprintf(`TTL %s + toIntervalHour(%d)`, timeField, ttl/time.Hour)
		case ttl%(time.Minute) == 0:
			return fmt.Sprintf(`TTL %s + toIntervalMinute(%d)`, timeField, ttl/time.Minute)
		default:
			return fmt.Sprintf(`TTL %s + toIntervalSecond(%d)`, timeField, ttl/time.Second)
		}
	}

	return ""
}
