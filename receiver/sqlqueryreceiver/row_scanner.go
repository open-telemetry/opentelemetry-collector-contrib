// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/multierr"
)

var errNullValueWarning = errors.New("NULL value")

type rowScanner struct {
	cols       map[string]func() (string, error)
	scanTarget []any
}

func newRowScanner(colTypes []colType) *rowScanner {
	rs := &rowScanner{
		cols: map[string]func() (string, error){},
	}
	for _, sqlType := range colTypes {
		colName := sqlType.Name()
		var v any
		rs.cols[colName] = func() (string, error) {
			if v == nil {
				return "", errNullValueWarning
			}
			format := "%v"
			if t, isTime := v.(time.Time); isTime {
				return t.Format(time.RFC3339), nil
			}
			if reflect.TypeOf(v).Kind() == reflect.Slice {
				// The Postgres driver returns a []uint8 (ascii string) for decimal and numeric types,
				// which we want to render as strings. e.g. "4.1" instead of "[52, 46, 49]".
				// Other slice types get the same treatment.
				format = "%s"
			}
			// turn whatever we got from the database driver into a string
			return fmt.Sprintf(format, v), nil
		}
		rs.scanTarget = append(rs.scanTarget, &v)
	}
	return rs
}

func (rs *rowScanner) scan(sqlRows rows) error {
	return sqlRows.Scan(rs.scanTarget...)
}

func (rs *rowScanner) toStringMap() (stringMap, error) {
	out := stringMap{}
	var errs error
	for k, f := range rs.cols {
		s, err := f()
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("column %q: %w", k, err))
		} else {
			out[k] = s
		}
	}
	return out, errs
}
