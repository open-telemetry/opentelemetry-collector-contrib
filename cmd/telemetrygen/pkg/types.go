// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"strings"
	"time"
)

// DurationWithInf is a custom type that can handle both regular durations and "inf" value
type DurationWithInf time.Duration

func (d *DurationWithInf) String() string {
	if *d == DurationWithInf(-1) {
		return "inf"
	}
	return time.Duration(*d).String()
}

func (d *DurationWithInf) Set(s string) error {
	if strings.EqualFold(s, "inf") {
		*d = DurationWithInf(-1)
		return nil
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = DurationWithInf(duration)
	return nil
}

func (*DurationWithInf) Type() string {
	return "duration|inf"
}

func (d *DurationWithInf) IsInf() bool {
	return *d == DurationWithInf(-1)
}

func (d *DurationWithInf) Duration() time.Duration {
	if d.IsInf() {
		return 0
	}
	return time.Duration(*d)
}
