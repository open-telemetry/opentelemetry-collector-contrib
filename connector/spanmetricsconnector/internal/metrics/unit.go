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
