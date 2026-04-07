// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal"

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	// ttlDeleteLimit is the max number of rows deleted per EVENT execution
	// to avoid long-running transactions.
	ttlDeleteLimit = 100000
)

// CreateTTLEvent creates a MySQL scheduled EVENT that periodically deletes
// expired rows from the given table.
//
// Parameters:
//   - database: the database name
//   - tableName: the table to clean up
//   - timeColumn: the DATETIME column used for expiration (e.g. "timestamp", "time_unix")
//   - ttl: data retention duration; if 0, no EVENT is created
func CreateTTLEvent(ctx context.Context, db *sql.DB, database, tableName, timeColumn string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	eventName := "ttl_" + tableName
	ttlSeconds := int64(ttl.Seconds())

	query := fmt.Sprintf(
		"CREATE EVENT IF NOT EXISTS `%s`.`%s` "+
			"ON SCHEDULE EVERY 1 HOUR "+
			"DO "+
			"DELETE FROM `%s`.`%s` WHERE `%s` < DATE_SUB(NOW(), INTERVAL %d SECOND) LIMIT %d",
		database, eventName,
		database, tableName, timeColumn, ttlSeconds, ttlDeleteLimit,
	)

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("create TTL event for %s: %w", tableName, err)
	}
	return nil
}
