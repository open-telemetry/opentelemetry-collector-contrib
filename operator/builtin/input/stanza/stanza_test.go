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

package stanza

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestStanzaOperator(t *testing.T) {
	cfg := NewInputConfig("test")
	cfg.OutputIDs = []string{"fake"}

	bc := testutil.NewBuildContext(t)
	ops, err := cfg.Build(bc)
	require.NoError(t, err)
	op := ops[0]

	fake := testutil.NewFakeOutput(t)
	op.SetOutputs([]operator.Operator{fake})

	require.NoError(t, op.Start())
	defer op.Stop()

	bc.Logger.Errorw("test failure", "key", "value")

	expectedRecord := map[string]interface{}{
		"message": "test failure",
		"key":     "value",
	}

	select {
	case e := <-fake.Received:
		require.Equal(t, expectedRecord, e.Record)

	case <-time.After(time.Second):
		require.FailNow(t, "timed out")
	}
}

func TestStanzaOperatorBUildFailure(t *testing.T) {
	cfg := NewInputConfig("")
	cfg.OperatorType = ""
	bc := testutil.NewBuildContext(t)
	_, err := cfg.Build(bc)
	require.Error(t, err)
}
