// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package models provides custom types for handling SQL Server specific data types
// and conversions used in performance monitoring and query analysis.
package models

import (
	"encoding/hex"
	"fmt"
)

// QueryID represents a SQL Server query_hash as a binary(8) field.
// It handles the conversion from SQL Server's binary query hash to a hex string representation.
type QueryID string

// Scan implements the sql.Scanner interface for QueryID.
// This method is called when SQL query results are scanned into the QueryID type.
// It converts SQL Server's binary(8) query_hash values into hex string format.
func (qid *QueryID) Scan(value interface{}) error {
	if value == nil {
		*qid = ""
		return nil
	}

	switch v := value.(type) {
	case []byte:
		// Handle []byte (which is alias for []uint8 - covers both cases)
		*qid = QueryID("0x" + hex.EncodeToString(v))
		return nil
	default:
		return fmt.Errorf("QueryID.Scan: cannot convert %T to QueryID", value)
	}
}

// String returns the hex string representation of the QueryID.
func (qid QueryID) String() string {
	return string(qid)
}

// IsEmpty checks if the QueryID is empty or null.
func (qid QueryID) IsEmpty() bool {
	return qid == "" || qid == "0x"
}