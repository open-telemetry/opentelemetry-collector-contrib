// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"errors"
	"fmt"
	"reflect"

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
