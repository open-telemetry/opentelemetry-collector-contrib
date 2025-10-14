package models

import "database/sql"

// BlockingQuery represents a blocking query record from Oracle V$SESSION views
type BlockingQuery struct {
	BlockedSID       sql.NullInt64
	BlockedSerial    sql.NullInt64
	BlockedUser      sql.NullString
	BlockedWaitSec   sql.NullFloat64
	BlockedSQLID     sql.NullString
	BlockedQueryText sql.NullString
	BlockingSID      sql.NullInt64
	BlockingSerial   sql.NullInt64
	BlockingUser     sql.NullString
	DatabaseName     sql.NullString
}

// GetBlockedUser returns the blocked user as a string, empty if null
func (bq *BlockingQuery) GetBlockedUser() string {
	if bq.BlockedUser.Valid {
		return bq.BlockedUser.String
	}
	return ""
}

// GetBlockedQueryText returns the blocked query text as a string, empty if null
func (bq *BlockingQuery) GetBlockedQueryText() string {
	if bq.BlockedQueryText.Valid {
		return bq.BlockedQueryText.String
	}
	return ""
}

// GetBlockingUser returns the blocking user as a string, empty if null
func (bq *BlockingQuery) GetBlockingUser() string {
	if bq.BlockingUser.Valid {
		return bq.BlockingUser.String
	}
	return ""
}

// GetBlockedSQLID returns the blocked SQL ID as a string, empty if null
func (bq *BlockingQuery) GetBlockedSQLID() string {
	if bq.BlockedSQLID.Valid {
		return bq.BlockedSQLID.String
	}
	return ""
}

// GetDatabaseName returns the database name as a string, empty if null
func (bq *BlockingQuery) GetDatabaseName() string {
	if bq.DatabaseName.Valid {
		return bq.DatabaseName.String
	}
	return ""
}

// HasValidWaitTime checks if the query has a valid wait time
func (bq *BlockingQuery) HasValidWaitTime() bool {
	return bq.BlockedWaitSec.Valid && bq.BlockedWaitSec.Float64 >= 0
}

// HasValidBlockedSQLID checks if the query has a valid blocked SQL ID
func (bq *BlockingQuery) HasValidBlockedSQLID() bool {
	return bq.BlockedSQLID.Valid
}

// IsValidForMetrics checks if the blocking query has the minimum required fields for metrics
func (bq *BlockingQuery) IsValidForMetrics() bool {
	return bq.HasValidBlockedSQLID() && bq.HasValidWaitTime()
}
