// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

const (
	MatchAll string = "true"
)

func TestTransformer(t *testing.T) {
	now := time.Now()
	t1 := time.Date(2020, time.April, 11, 21, 34, 01, 0, time.UTC)
	t2 := time.Date(2020, time.April, 11, 21, 34, 02, 0, time.UTC)

	entryWithBody := func(ts time.Time, body interface{}) *entry.Entry {
		e := entry.New()
		e.ObservedTimestamp = now
		e.Timestamp = ts
		e.Body = body
		return e
	}

	entryWithBodyAttr := func(ts time.Time, body interface{}, Attr map[string]string) *entry.Entry {
		e := entryWithBody(ts, body)
		for k, v := range Attr {
			e.AddAttribute(k, v)
		}
		return e
	}

	cases := []struct {
		name           string
		config         *Config
		input          []*entry.Entry
		expectedOutput []*entry.Entry
	}{
		{
			"NoEntriesFirst",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = MatchAll
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			nil,
			nil,
		},
		{
			"NoEntriesLast",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = MatchAll
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			nil,
			nil,
		},
		{
			"OneEntryFirst",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = MatchAll
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{entry.New()},
			nil,
		},
		{
			"OneEntryLast",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = MatchAll
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{entryWithBody(t1, "test")},
			[]*entry.Entry{entryWithBody(t1, "test")},
		},
		{
			"TwoEntriesLast",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'test2'"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test1"),
				entryWithBody(t2, "test2"),
			},
			[]*entry.Entry{entryWithBody(t1, "test1\ntest2")},
		},
		{
			"ThreeEntriesFirstNewest",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.OverwriteWith = "newest"
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test1"),
				entryWithBody(t2, "test2"),
				entryWithBody(t2, "test1"),
			},
			[]*entry.Entry{
				entryWithBody(t2, "test1\ntest2"),
			},
		},
		{
			"EntriesNonMatchingForFirstEntry",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "$body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.OverwriteWith = "newest"
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t2, "test3"),
				entryWithBody(t2, "test4"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t2, "test3"),
				entryWithBody(t2, "test4"),
			},
		},
		{
			"EntriesMatchingForFirstEntryOneFileOnly",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'file1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.OverwriteWith = "newest"
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file3", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t2, "file2", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t2, "file2", map[string]string{"file.path": "file2"}),
				entryWithBodyAttr(t2, "file3", map[string]string{"file.path": "file2"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1\nfile3", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t2, "file1\nfile2", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t2, "file2", map[string]string{"file.path": "file2"}),
				entryWithBodyAttr(t2, "file3", map[string]string{"file.path": "file2"}),
			},
		},
		{
			"CombineWithEmptyString",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.CombineWith = ""
				cfg.IsLastEntry = "body == 'test2'"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test1"),
				entryWithBody(t1, "test2"),
			},
			[]*entry.Entry{entryWithBody(t1, "test1test2")},
		},
		{
			"TestDefaultSourceIdentifier",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'end'"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file2", map[string]string{"file.path": "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t2, "end", map[string]string{"file.path": "file2"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1\nend", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file2\nend", map[string]string{"file.path": "file2"}),
			},
		},
		{
			"TestCustomSourceIdentifier",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'end'"
				cfg.OutputIDs = []string{"fake"}
				cfg.SourceIdentifier = entry.NewAttributeField("custom_source")
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1", map[string]string{"custom_source": "file1"}),
				entryWithBodyAttr(t1, "file2", map[string]string{"custom_source": "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{"custom_source": "file1"}),
				entryWithBodyAttr(t2, "end", map[string]string{"custom_source": "file2"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1\nend", map[string]string{"custom_source": "file1"}),
				entryWithBodyAttr(t1, "file2\nend", map[string]string{"custom_source": "file2"}),
			},
		},
		{
			"TestMaxSources",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'end'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxSources = 1
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t2, "end", map[string]string{"file.path": "file1"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t2, "end", map[string]string{"file.path": "file1"}),
			},
		},
		{
			"TestMaxBatchSize",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'end'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxBatchSize = 2
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1_event1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file2_event1", map[string]string{"file.path": "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t2, "file2_event2", map[string]string{"file.path": "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{"file.path": "file2"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1_event1\nend", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file2_event1\nfile2_event2", map[string]string{"file.path": "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{"file.path": "file2"}),
			},
		},
		{
			"TestMaxLogSizeForLastEntry",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'end'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxLogSize = helper.ByteSize(5)
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file2", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "end", map[string]string{"file.path": "file1"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1\nfile1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "file2\nend", map[string]string{"file.path": "file1"}),
			},
		},
		{
			"TestMaxLogSizeForFirstEntry",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'start'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxLogSize = helper.ByteSize(12)
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content2", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content3", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content4", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content5", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "start", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "start", map[string]string{"file.path": "file1"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start\ncontent1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content2\ncontent3", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content4\ncontent5", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "start", map[string]string{"file.path": "file1"}),
			},
		},
		{
			"TestBatchSplitWhenTriggerTheBatchSizeLimit",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'start'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxBatchSize = 5
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content1", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content2", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content3", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content4", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content5", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content6", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content7", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content8", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content9", map[string]string{"file.path": "file1"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start\ncontent1\ncontent2\ncontent3\ncontent4", map[string]string{"file.path": "file1"}),
				entryWithBodyAttr(t1, "content5\ncontent6\ncontent7\ncontent8\ncontent9", map[string]string{"file.path": "file1"}),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := tc.config.Build(testutil.Logger(t))
			require.NoError(t, err)
			recombine := op.(*Transformer)

			fake := testutil.NewFakeOutput(t)
			err = recombine.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			for _, e := range tc.input {
				require.NoError(t, recombine.Process(context.Background(), e))
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
		cfg := NewConfig()
		cfg.CombineField = entry.NewBodyField()
		cfg.IsFirstEntry = MatchAll
		cfg.OutputIDs = []string{"fake"}
		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)
		recombine := op.(*Transformer)

		fake := testutil.NewFakeOutput(t)
		err = recombine.SetOutputs([]operator.Operator{fake})
		require.NoError(t, err)

		// Send an entry that isn't the last in a multiline
		require.NoError(t, recombine.Process(context.Background(), entry.New()))

		// Ensure that the entry isn't immediately sent
		select {
		case <-fake.Received:
			require.FailNow(t, "Received unexpected entry")
		case <-time.After(10 * time.Millisecond):
		}

		// Stop the operator
		require.NoError(t, recombine.Stop())

		// Ensure that the entries in the buffer are flushed
		select {
		case <-fake.Received:
		default:
			require.FailNow(t, "Entry was not flushed on shutdown")
		}
	})
}

func BenchmarkRecombine(b *testing.B) {
	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsFirstEntry = "body startsWith 'log-0'"
	cfg.OutputIDs = []string{"fake"}
	cfg.SourceIdentifier = entry.NewAttributeField("file.path")
	op, err := cfg.Build(testutil.Logger(b))
	require.NoError(b, err)
	recombine := op.(*Transformer)

	fake := testutil.NewFakeOutput(b)
	require.NoError(b, recombine.SetOutputs([]operator.Operator{fake}))

	go func() {
		for {
			<-fake.Received
		}
	}()

	sourcesNum := 10
	logsNum := 10
	entries := []*entry.Entry{}
	for i := 0; i < logsNum; i++ {
		for j := 0; j < sourcesNum; j++ {
			start := entry.New()
			start.Timestamp = time.Now()
			start.Body = strings.Repeat(fmt.Sprintf("log-%d", i), 50)
			start.Attributes = map[string]any{"file.path": fmt.Sprintf("file-%d", j)}
			entries = append(entries, start)
		}
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, e := range entries {
			require.NoError(b, recombine.Process(ctx, e))
		}
		recombine.flushUncombined(ctx)
	}
}

func BenchmarkRecombineLimitTrigger(b *testing.B) {
	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsFirstEntry = "body == 'start'"
	cfg.MaxLogSize = 6
	cfg.OutputIDs = []string{"fake"}
	op, err := cfg.Build(testutil.Logger(b))
	require.NoError(b, err)
	recombine := op.(*Transformer)

	fake := testutil.NewFakeOutput(b)
	require.NoError(b, recombine.SetOutputs([]operator.Operator{fake}))
	require.NoError(b, recombine.Start(nil))

	go func() {
		for {
			<-fake.Received
		}
	}()

	start := entry.New()
	start.Timestamp = time.Now()
	start.Body = "start"

	next := entry.New()
	next.Timestamp = time.Now()
	next.Body = "next"

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, recombine.Process(ctx, start))
		require.NoError(b, recombine.Process(ctx, next))
		require.NoError(b, recombine.Process(ctx, start))
		require.NoError(b, recombine.Process(ctx, next))
		recombine.flushUncombined(ctx)
	}

}

func TestTimeout(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsFirstEntry = MatchAll
	cfg.OutputIDs = []string{"fake"}
	cfg.ForceFlushTimeout = 100 * time.Millisecond
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	recombine := op.(*Transformer)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, recombine.SetOutputs([]operator.Operator{fake}))

	e := entry.New()
	e.Timestamp = time.Now()
	e.Body = "body"

	ctx := context.Background()

	require.NoError(t, recombine.Start(nil))
	require.NoError(t, recombine.Process(ctx, e))
	select {
	case <-fake.Received:
		t.Logf("We shouldn't receive an entry before timeout")
		t.FailNow()
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case <-fake.Received:
	case <-time.After(5 * time.Second):
		t.Logf("The entry should be flushed by now")
		t.FailNow()
	}

	require.NoError(t, recombine.Stop())
}

// This test is to make sure the timeout would take effect when there
// are constantly logs that meet the aggregation criteria
func TestTimeoutWhenAggregationKeepHappen(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsFirstEntry = "body == 'start'"
	cfg.CombineWith = ""
	cfg.OutputIDs = []string{"fake"}
	cfg.ForceFlushTimeout = 100 * time.Millisecond
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	recombine := op.(*Transformer)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, recombine.SetOutputs([]operator.Operator{fake}))

	e := entry.New()
	e.Timestamp = time.Now()
	e.Body = "start"

	ctx := context.Background()

	require.NoError(t, recombine.Start(nil))
	require.NoError(t, recombine.Process(ctx, e))

	go func() {
		next := e.Copy()
		next.Body = "next"
		for {
			time.Sleep(cfg.ForceFlushTimeout / 2)
			require.NoError(t, recombine.Process(ctx, next))
		}
	}()

	select {
	case <-fake.Received:
	case <-time.After(5 * time.Second):
		t.Logf("The entry should be flushed by now")
		t.FailNow()
	}
	require.NoError(t, recombine.Stop())
}

func TestSourceBatchDelete(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsFirstEntry = "body == 'start'"
	cfg.OutputIDs = []string{"fake"}
	cfg.ForceFlushTimeout = 100 * time.Millisecond
	cfg.MaxLogSize = 6
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	recombine := op.(*Transformer)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, recombine.SetOutputs([]operator.Operator{fake}))

	start := entry.New()
	start.Timestamp = time.Now()
	start.Body = "start"
	start.AddAttribute("file.path", "file1")

	next := entry.New()
	next.Timestamp = time.Now()
	next.Body = "next"
	start.AddAttribute("file.path", "file1")

	ctx := context.Background()

	require.NoError(t, recombine.Process(ctx, start))
	require.NoError(t, recombine.Process(ctx, next))
	require.Equal(t, 1, len(recombine.batchMap))
	require.NoError(t, recombine.flushSource("file1", true))
	require.Equal(t, 0, len(recombine.batchMap))
	require.NoError(t, recombine.Stop())
}
