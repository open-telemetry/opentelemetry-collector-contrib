// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"

import (
	"encoding"
	"errors"
	"fmt"
	"strings"
)

const (
	Milliseconds Unit = iota
	Seconds

	MillisecondsStr = "ms"
	SecondsStr      = "s"
)

type Unit int8

var _ encoding.TextMarshaler = (*Unit)(nil)
var _ encoding.TextUnmarshaler = (*Unit)(nil)

func (u Unit) String() string {
	switch u {
	case Milliseconds:
		return MillisecondsStr
	case Seconds:
		return SecondsStr
	}
	return ""
}

// MarshalText marshals Unit to text.
func (u Unit) MarshalText() (text []byte, err error) {
	return []byte(u.String()), nil
}

// UnmarshalText unmarshalls text to a Unit.
func (u *Unit) UnmarshalText(text []byte) error {
	if u == nil {
		return errors.New("cannot unmarshal to a nil *Unit")
	}

	str := strings.ToLower(string(text))
	switch str {
	case strings.ToLower(MillisecondsStr):
		*u = Milliseconds
		return nil
	case strings.ToLower(SecondsStr):
		*u = Seconds
		return nil
	}
	return fmt.Errorf("unknown Unit %q, allowed units are %q and %q", str, MillisecondsStr, SecondsStr)
}
