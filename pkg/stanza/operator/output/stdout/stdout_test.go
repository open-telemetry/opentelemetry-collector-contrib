// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

func TestOperator(t *testing.T) {
	cfg := Config{
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
	op.(*Output).encoder = json.NewEncoder(&buf)

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
