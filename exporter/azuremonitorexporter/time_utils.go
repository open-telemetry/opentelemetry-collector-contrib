// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

// Contains code common to both trace and metrics exporters

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func toTime(t pcommon.Timestamp) time.Time {
	return time.Unix(0, int64(t))
}

// Formats a Duration into the form DD.HH:MM:SS.MMMMMM
func formatDuration(d time.Duration) string {
	day := d / (time.Hour * 24)
	d -= day * (time.Hour * 24)

	h := d / time.Hour
	d -= h * time.Hour

	m := d / time.Minute
	d -= m * time.Minute

	s := d / time.Second
	d -= s * time.Second

	us := (d / time.Microsecond)

	return fmt.Sprintf("%02d.%02d:%02d:%02d.%06d", day, h, m, s, us)
}

var timeNow = time.Now
