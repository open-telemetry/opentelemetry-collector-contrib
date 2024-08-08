// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Keep the original license.

// Copyright 2019 Dmitry A. Mottl. All rights reserved.
// Use of this source code is governed by MIT license
// that can be found in the LICENSE file.

package ctimefmt

import (
	"testing"
	"time"
)

var format1 = "%Y-%m-%d %H:%M:%S.%f"
var format2 = "%Y-%m-%d %l:%M:%S.%L %P, %a"
var value1 = "2019-01-02 15:04:05.666666"
var value2 = "2019-01-02 3:04:05.666 pm, Wed"
var dt1 = time.Date(2019, 1, 2, 15, 4, 5, 666666000, time.UTC)
var dt2 = time.Date(2019, 1, 2, 15, 4, 5, 666000000, time.UTC)

func TestFormat(t *testing.T) {
	s, err := Format(format1, dt1)
	if err != nil {
		t.Fatal(err)
	}
	if s != value1 {
		t.Errorf("Given: %v, expected: %v", s, value1)
	}

	s, err = Format(format2, dt1)
	if err != nil {
		t.Fatal(err)
	}
	if s != value2 {
		t.Errorf("Given: %v, expected: %v", s, value2)
	}
}

func TestParse(t *testing.T) {
	dt, err := Parse(format1, value1)
	if err != nil {
		t.Error(err)
	} else if dt != dt1 {
		t.Errorf("Given: %v, expected: %v", dt, dt1)
	}

	dt, err = Parse(format2, value2)
	if err != nil {
		t.Error(err)
	} else if dt != dt2 {
		t.Errorf("Given: %v, expected: %v", dt, dt2)
	}
}

func TestZulu(t *testing.T) {
	format := "%Y-%m-%dT%H:%M:%S.%L%z"
	// These time should all parse as UTC.
	for _, input := range []string{
		"2019-01-02T15:04:05.666666Z",
		"2019-01-02T15:04:05.666666-0000",
		"2019-01-02T15:04:05.666666+0000",
	} {
		t.Run(input, func(t *testing.T) {
			dt, err := Parse(format, input)
			if err != nil {
				t.Error(err)
			} else if dt.UnixNano() != dt1.UnixNano() {
				// We compare the unix nanoseconds because Go has a subtle parsing difference between "Z" and "+0000".
				// The former returns a Time with the UTC timezone, the latter returns a Time with a 0000 time zone offset.
				// (See Go's documentation for `time.Parse`.)
				t.Errorf("Given: %v, expected: %v", dt, dt1)
			}
		})
	}
}
