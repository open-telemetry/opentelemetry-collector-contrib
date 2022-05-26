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

package stdout

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestStdoutOperator(t *testing.T) {
	cfg := StdoutConfig{
		OutputConfig: helper.OutputConfig{
			BasicConfig: helper.BasicConfig{
				OperatorID:   "test_operator_id",
				OperatorType: "stdout",
			},
		},
	}

	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	var buf bytes.Buffer
	op.(*StdoutOperator).encoder = json.NewEncoder(&buf)

	ots := time.Unix(1591043864, 0)
	ts := time.Unix(1591042864, 0)
	e := &entry.Entry{
		ObservedTimestamp: ots,
		Timestamp:         ts,
		Body:              "test body",
	}
	err = op.Process(context.Background(), e)
	require.NoError(t, err)

	marshalledOTS, err := json.Marshal(ots)
	require.NoError(t, err)

	marshalledTs, err := json.Marshal(ts)
	require.NoError(t, err)

	expected := `{"observed_timestamp":` + string(marshalledOTS) + `,"timestamp":` + string(marshalledTs) + `,"body":"test body","severity":0,"scope_name":""}` + "\n"
	require.Equal(t, expected, buf.String())
}
