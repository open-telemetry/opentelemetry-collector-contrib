// Copyright 2019, OpenTelemetry Authors
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

package azuremonitorexporter

import (
	"testing"
	"time"

	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
)

func TestToTime(t *testing.T) {
	// 10 seconds after the Unix epoch of 1970-01-01T00:00:00Z
	input := &timestamp.Timestamp{
		Seconds: 60,
		Nanos:   1,
	}

	output := toTime(input)

	assert.NotNil(t, output)
	expected := time.Date(1970, 01, 01, 00, 01, 00, 1, time.UTC)
	assert.Equal(t, "1970-01-01T00:01:00.000000001Z", expected.Format(time.RFC3339Nano))
}

func TestFormatDuration(t *testing.T) {
	// 1 minute 30 second duration
	dur := time.Minute * 1
	dur += time.Second * 30

	output := formatDuration(dur)

	// DD.HH:MM:SS.MMMMMM
	assert.NotNil(t, output)
	assert.Equal(t, "00.00:01:30.000000", output)
}
