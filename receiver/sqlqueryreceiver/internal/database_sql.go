// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal"

import (
	"database/sql"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
)

func NewPool(opener sqlquery.SQLOpenerFunc, driver string, dsn string, maxOpenConns int) interface {
	DB() (*sql.DB, error)
} {
	return &sqlPool{
		DriverName:     driver,
		DataSourceName: dsn,
		MaxOpenConns:   maxOpenConns,
		Opener:         opener,
	}
}

type sqlPool struct {
	DriverName     string
	DataSourceName string
	MaxOpenConns   int
	Opener         sqlquery.SQLOpenerFunc

	mutex sync.Mutex
	db    *sql.DB
}

// DB initializes and returns the [sql.DB] pool. It is safe to call concurrently.
// Depending on the driver, this might also connect to the database backend.
//
// This method exists to satisfy [sqlquery.DbProviderFunc], but the way
// [sqlquery.Scraper] closes [sql.DB] can interfere with other Scrapers.
func (sp *sqlPool) DB() (*sql.DB, error) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	if sp.db == nil {
		db, err := sp.Opener(sp.DriverName, sp.DataSourceName)

		if err == nil && db != nil {
			db.SetMaxOpenConns(sp.MaxOpenConns)
			sp.db = db
		}

		return sp.db, err
	}

	return sp.db, nil
}
