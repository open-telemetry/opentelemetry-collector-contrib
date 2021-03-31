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

package recombine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestRecombineOperator(t *testing.T) {
	t1 := time.Date(2020, time.April, 11, 21, 34, 01, 0, time.UTC)
	t2 := time.Date(2020, time.April, 11, 21, 34, 02, 0, time.UTC)

	entryWithRecord := func(ts time.Time, record interface{}) *entry.Entry {
		e := entry.New()
		e.Timestamp = ts
		e.Record = record
		return e
	}

	cases := []struct {
		name           string
		config         *RecombineOperatorConfig
		input          []*entry.Entry
		expectedOutput []*entry.Entry
	}{
		{
			"NoEntriesFirst",
			func() *RecombineOperatorConfig {
				cfg := NewRecombineOperatorConfig("")
				cfg.CombineField = entry.NewRecordField()
				cfg.IsFirstEntry = "true"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			nil,
			nil,
		},
		{
			"NoEntriesLast",
			func() *RecombineOperatorConfig {
				cfg := NewRecombineOperatorConfig("")
				cfg.CombineField = entry.NewRecordField()
				cfg.IsLastEntry = "true"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			nil,
			nil,
		},
		{
			"OneEntryFirst",
			func() *RecombineOperatorConfig {
				cfg := NewRecombineOperatorConfig("")
				cfg.CombineField = entry.NewRecordField()
				cfg.IsFirstEntry = "true"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{entry.New()},
			nil,
		},
		{
			"OneEntryLast",
			func() *RecombineOperatorConfig {
				cfg := NewRecombineOperatorConfig("")
				cfg.CombineField = entry.NewRecordField()
				cfg.IsLastEntry = "true"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{entryWithRecord(t1, "test")},
			[]*entry.Entry{entryWithRecord(t1, "test")},
		},
		{
			"TwoEntriesLast",
			func() *RecombineOperatorConfig {
				cfg := NewRecombineOperatorConfig("")
				cfg.CombineField = entry.NewRecordField()
				cfg.IsLastEntry = "$record == 'test2'"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{
				entryWithRecord(t1, "test1"),
				entryWithRecord(t2, "test2"),
			},
			[]*entry.Entry{entryWithRecord(t1, "test1\ntest2")},
		},
		{
			"ThreeEntriesFirstNewest",
			func() *RecombineOperatorConfig {
				cfg := NewRecombineOperatorConfig("")
				cfg.CombineField = entry.NewRecordField()
				cfg.IsFirstEntry = "$record == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.OverwriteWith = "newest"
				return cfg
			}(),
			[]*entry.Entry{
				entryWithRecord(t1, "test1"),
				entryWithRecord(t2, "test2"),
				entryWithRecord(t2, "test1"),
			},
			[]*entry.Entry{
				entryWithRecord(t2, "test1\ntest2"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops, err := tc.config.Build(testutil.NewBuildContext(t))
			require.NoError(t, err)
			recombine := ops[0].(*RecombineOperator)

			fake := testutil.NewFakeOutput(t)
			err = recombine.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			for _, e := range tc.input {
				recombine.Process(context.Background(), e)
			}

			for _, expected := range tc.expectedOutput {
				fake.ExpectEntry(t, expected)
			}

			select {
			case e := <-fake.Received:
				require.FailNow(t, "Received unexpected entry: ", e)
			default:
			}
		})
	}

	t.Run("FlushesOnShutdown", func(t *testing.T) {
		cfg := NewRecombineOperatorConfig("")
		cfg.CombineField = entry.NewRecordField()
		cfg.IsFirstEntry = "false"
		cfg.OutputIDs = []string{"fake"}
		ops, err := cfg.Build(testutil.NewBuildContext(t))
		require.NoError(t, err)
		recombine := ops[0].(*RecombineOperator)

		fake := testutil.NewFakeOutput(t)
		err = recombine.SetOutputs([]operator.Operator{fake})
		require.NoError(t, err)

		// Send an entry that isn't the last in a multiline
		recombine.Process(context.Background(), entry.New())

		// Ensure that the entry isn't immediately sent
		select {
		case <-fake.Received:
			require.FailNow(t, "Received unexpected entry")
		case <-time.After(10 * time.Millisecond):
		}

		// Stop the operator
		recombine.Stop()

		// Ensure that the entries in the buffer are flushed
		select {
		case <-fake.Received:
		default:
			require.FailNow(t, "Entry was not flushed on shutdown")
		}
	})
}
