// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

const (
	MatchAll string = "true"
)

func TestTransformer(t *testing.T) {
	now := time.Now()
	t1 := time.Date(2020, time.April, 11, 21, 34, 0o1, 0, time.UTC)
	t2 := time.Date(2020, time.April, 11, 21, 34, 0o2, 0, time.UTC)

	entryWithBody := func(ts time.Time, body any) *entry.Entry {
		e := entry.New()
		e.ObservedTimestamp = now
		e.Timestamp = ts
		e.Body = body
		return e
	}

	entryWithBodyAttr := func(ts time.Time, body any, Attr map[string]string) *entry.Entry {
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
				entryWithBodyAttr(t1, "test1", map[string]string{"base": "false"}),
				entryWithBodyAttr(t2, "test2", map[string]string{"base": "true"}),
				entryWithBodyAttr(t2, "test1", map[string]string{"base": "false"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t2, "test1\ntest2", map[string]string{"base": "true"}),
			},
		},
		{
			"ThreeEntriesFirstOldest",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.OverwriteWith = "oldest"
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "test1", map[string]string{"base": "true"}),
				entryWithBodyAttr(t2, "test2", map[string]string{"base": "false"}),
				entryWithBodyAttr(t2, "test1", map[string]string{"base": "true"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "test1\ntest2", map[string]string{"base": "true"}),
			},
		},
		{
			"EntriesNonMatchingForFirstEntry",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "$body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.OverwriteWith = "oldest"
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t2, "test3"),
				entryWithBody(t2, "test4"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2\ntest3\ntest4"),
			},
		},
		{
			"EntriesMatchingForFirstEntryOneFileOnly",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'start'"
				cfg.OutputIDs = []string{"fake"}
				cfg.OverwriteWith = "newest"
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "more1a", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "start", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "more1b", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "start", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "more2a", map[string]string{attrs.LogFilePath: "file2"}),
				entryWithBodyAttr(t2, "more2b", map[string]string{attrs.LogFilePath: "file2"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start\nmore1a", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "start\nmore1b", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "start", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "more2a\nmore2b", map[string]string{attrs.LogFilePath: "file2"}),
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
			"Stacktrace",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = `body matches "^[^\\s]"`
				cfg.OutputIDs = []string{"fake"}
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "Log message 1"),
				entryWithBody(t1, "Error: java.lang.Exception: Stack trace"),
				entryWithBody(t1, "        at java.lang.Thread.dumpStack(Thread.java:1336)"),
				entryWithBody(t1, "        at Main.demo3(Main.java:15)"),
				entryWithBody(t1, "        at Main.demo2(Main.java:12)"),
				entryWithBody(t1, "        at Main.demo1(Main.java:9)"),
				entryWithBody(t1, "        at Main.demo(Main.java:6)"),
				entryWithBody(t1, "        at Main.main(Main.java:3)"),
				entryWithBody(t1, "Another log message"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "Log message 1"),
				entryWithBody(t1, "Error: java.lang.Exception: Stack trace\n"+
					"        at java.lang.Thread.dumpStack(Thread.java:1336)\n"+
					"        at Main.demo3(Main.java:15)\n"+
					"        at Main.demo2(Main.java:12)\n"+
					"        at Main.demo1(Main.java:9)\n"+
					"        at Main.demo(Main.java:6)\n"+
					"        at Main.main(Main.java:3)"),
				entryWithBody(t1, "Another log message"),
			},
		},
		{
			"StacktraceSubfield",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField("message")
				cfg.IsFirstEntry = `body.message matches "^[^\\s]"`
				cfg.OutputIDs = []string{"fake"}
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, map[string]any{"message": "Log message 1"}),
				entryWithBody(t1, map[string]any{"message": "Error: java.lang.Exception: Stack trace"}),
				entryWithBody(t1, map[string]any{"message": "        at java.lang.Thread.dumpStack(Thread.java:1336)"}),
				entryWithBody(t1, map[string]any{"message": "        at Main.demo3(Main.java:15)"}),
				entryWithBody(t1, map[string]any{"message": "        at Main.demo2(Main.java:12)"}),
				entryWithBody(t1, map[string]any{"message": "        at Main.demo1(Main.java:9)"}),
				entryWithBody(t1, map[string]any{"message": "        at Main.demo(Main.java:6)"}),
				entryWithBody(t1, map[string]any{"message": "        at Main.main(Main.java:3)"}),
				entryWithBody(t1, map[string]any{"message": "Another log message"}),
			},
			[]*entry.Entry{
				entryWithBody(t1, map[string]any{"message": "Log message 1"}),
				entryWithBody(t1, map[string]any{"message": "Error: java.lang.Exception: Stack trace\n" +
					"        at java.lang.Thread.dumpStack(Thread.java:1336)\n" +
					"        at Main.demo3(Main.java:15)\n" +
					"        at Main.demo2(Main.java:12)\n" +
					"        at Main.demo1(Main.java:9)\n" +
					"        at Main.demo(Main.java:6)\n" +
					"        at Main.main(Main.java:3)"}),
				entryWithBody(t1, map[string]any{"message": "Another log message"}),
			},
		},
		{
			"CombineSplitUnicode",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField("message")
				cfg.CombineWith = ""
				cfg.IsLastEntry = "body.logtag == 'F'"
				cfg.OverwriteWith = "newest"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, map[string]any{
					"message":   "Single entry log 1",
					"logtag":    "F",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:09.669794202Z",
				}),
				entryWithBody(t1, map[string]any{
					"message":   "\xe5\xbe",
					"logtag":    "P",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:10.113242941Z",
				}),
				entryWithBody(t1, map[string]any{
					"message":   "\x90",
					"logtag":    "F",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:10.113242941Z",
				}),
			},
			[]*entry.Entry{
				entryWithBody(t1, map[string]any{
					"message":   "Single entry log 1",
					"logtag":    "F",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:09.669794202Z",
				}),
				entryWithBody(t1, map[string]any{
					"message":   "徐",
					"logtag":    "F",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:10.113242941Z",
				}),
			},
		},
		{
			"CombineOtherThanCondition",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField("message")
				cfg.CombineWith = ""
				cfg.IsLastEntry = "body.logtag == 'F'"
				cfg.OverwriteWith = "newest"
				cfg.OutputIDs = []string{"fake"}
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, map[string]any{
					"message":   "Single entry log 1",
					"logtag":    "F",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:09.669794202Z",
				}),
				entryWithBody(t1, map[string]any{
					"message":   "This is a very very long line th",
					"logtag":    "P",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:10.113242941Z",
				}),
				entryWithBody(t1, map[string]any{
					"message":   "at is really really long and spa",
					"logtag":    "P",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:10.113242941Z",
				}),
				entryWithBody(t1, map[string]any{
					"message":   "ns across multiple log entries",
					"logtag":    "F",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:10.113242941Z",
				}),
			},
			[]*entry.Entry{
				entryWithBody(t1, map[string]any{
					"message":   "Single entry log 1",
					"logtag":    "F",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:09.669794202Z",
				}),
				entryWithBody(t1, map[string]any{
					"message":   "This is a very very long line that is really really long and spans across multiple log entries",
					"logtag":    "F",
					"stream":    "stdout",
					"timestamp": "2016-10-06T00:17:10.113242941Z",
				}),
			},
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
				entryWithBodyAttr(t1, "file1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "file2", map[string]string{attrs.LogFilePath: "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "end", map[string]string{attrs.LogFilePath: "file2"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1\nend", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "file2\nend", map[string]string{attrs.LogFilePath: "file2"}),
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
				cfg.OverwriteWith = "newest"
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1.Add(10*time.Millisecond), "middle1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "start2", map[string]string{attrs.LogFilePath: "file2"}),
				entryWithBodyAttr(t2.Add(10*time.Millisecond), "middle2", map[string]string{attrs.LogFilePath: "file2"}),
				entryWithBodyAttr(t2.Add(20*time.Millisecond), "end2", map[string]string{attrs.LogFilePath: "file2"}),
			},
			[]*entry.Entry{
				// First entry is booted before end comes in, but partial recombination should occur
				entryWithBodyAttr(t1.Add(10*time.Millisecond), "start1\nmiddle1", map[string]string{attrs.LogFilePath: "file1"}),
				// Second entry is flushed automatically when end comes in
				entryWithBodyAttr(t2.Add(20*time.Millisecond), "start2\nmiddle2\nend2", map[string]string{attrs.LogFilePath: "file2"}),
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
				entryWithBodyAttr(t1, "file1_event1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "file2_event1", map[string]string{attrs.LogFilePath: "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "file2_event2", map[string]string{attrs.LogFilePath: "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{attrs.LogFilePath: "file2"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1_event1\nend", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "file2_event1\nfile2_event2", map[string]string{attrs.LogFilePath: "file2"}),
				entryWithBodyAttr(t2, "end", map[string]string{attrs.LogFilePath: "file2"}),
			},
		},
		{
			"TestMaxBatchSizeUnlimited",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'end'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxBatchSize = 0 // unlimited
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBodyAttr(t1, "event1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "event2", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "event3", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "event4", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "event5", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t2, "end", map[string]string{attrs.LogFilePath: "file1"}),
			},
			[]*entry.Entry{
				// All entries combined into one because MaxBatchSize=0 means unlimited
				entryWithBodyAttr(t1, "event1\nevent2\nevent3\nevent4\nevent5\nend", map[string]string{attrs.LogFilePath: "file1"}),
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
				entryWithBodyAttr(t1, "file1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "file1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "file2", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "end", map[string]string{attrs.LogFilePath: "file1"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "file1\nfile1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "file2\nend", map[string]string{attrs.LogFilePath: "file1"}),
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
				entryWithBodyAttr(t1, "start", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content2", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content3", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content4", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content5", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "start", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "start", map[string]string{attrs.LogFilePath: "file1"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start\ncontent1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content2\ncontent3", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content4\ncontent5", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "start", map[string]string{attrs.LogFilePath: "file1"}),
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
				entryWithBodyAttr(t1, "start", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content1", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content2", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content3", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content4", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content5", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content6", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content7", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content8", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content9", map[string]string{attrs.LogFilePath: "file1"}),
			},
			[]*entry.Entry{
				entryWithBodyAttr(t1, "start\ncontent1\ncontent2\ncontent3\ncontent4", map[string]string{attrs.LogFilePath: "file1"}),
				entryWithBodyAttr(t1, "content5\ncontent6\ncontent7\ncontent8\ncontent9", map[string]string{attrs.LogFilePath: "file1"}),
			},
		},
		{
			"EntriesNonMatchingForFirstEntryWithMaxUnmatchedBatchSize=0",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxUnmatchedBatchSize = 0
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2\ntest3\ntest4"),
			},
		},
		{
			"EntriesNonMatchingForFirstEntryWithMaxUnmatchedBatchSize=1",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxUnmatchedBatchSize = 1
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
			},
		},
		{
			"TestMaxUnmatchedBatchSizeForFirstEntry",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsFirstEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxUnmatchedBatchSize = 2
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
				entryWithBody(t1, "test5"),
				entryWithBody(t1, "test6"),
				entryWithBody(t1, "test1"),
				entryWithBody(t1, "test7"),
				entryWithBody(t1, "test8"),
				entryWithBody(t1, "test1"),
				entryWithBody(t1, "test9"),
				entryWithBody(t1, "test10"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2\ntest3"),
				entryWithBody(t1, "test4\ntest5"),
				entryWithBody(t1, "test6"),
				entryWithBody(t1, "test1\ntest7\ntest8"),
				entryWithBody(t1, "test1\ntest9\ntest10"),
			},
		},
		{
			"EntriesNonMatchingForLastEntryWithMaxUnmatchedBatchSize=0",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxUnmatchedBatchSize = 0
				cfg.ForceFlushTimeout = 10 * time.Millisecond
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2\ntest3\ntest4"),
			},
		},
		{
			"EntriesNonMatchingForLastEntryWithMaxUnmatchedBatchSize=1",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxUnmatchedBatchSize = 1
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
			},
		},
		{
			"EntriesMatchingForLastEntryMaxUnmatchedBatchSize=2",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxUnmatchedBatchSize = 2
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
				entryWithBody(t1, "test5"),
				entryWithBody(t1, "test1"),
				entryWithBody(t1, "test6"),
				entryWithBody(t1, "test7"),
				entryWithBody(t1, "test1"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2\ntest3"),
				entryWithBody(t1, "test4\ntest5"),
				entryWithBody(t1, "test1"),
				entryWithBody(t1, "test6\ntest7"),
				entryWithBody(t1, "test1"),
			},
		},
		{
			"EntriesMatchingForLastEntryMaxUnmatchedBatchSize=3",
			func() *Config {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "body == 'test1'"
				cfg.OutputIDs = []string{"fake"}
				cfg.MaxUnmatchedBatchSize = 3
				return cfg
			}(),
			[]*entry.Entry{
				entryWithBody(t1, "test2"),
				entryWithBody(t1, "test3"),
				entryWithBody(t1, "test4"),
				entryWithBody(t1, "test5"),
				entryWithBody(t1, "test1"),
				entryWithBody(t1, "test6"),
				entryWithBody(t1, "test7"),
				entryWithBody(t1, "test1"),
			},
			[]*entry.Entry{
				entryWithBody(t1, "test2\ntest3\ntest4"),
				entryWithBody(t1, "test5\ntest1"),
				entryWithBody(t1, "test6\ntest7\ntest1"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			set := componenttest.NewNopTelemetrySettings()
			op, err := tc.config.Build(set)
			require.NoError(t, err)
			require.NoError(t, op.Start(testutil.NewUnscopedMockPersister()))
			defer func() { require.NoError(t, op.Stop()) }()

			fake := testutil.NewFakeOutput(t)
			err = op.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			for _, e := range tc.input {
				require.NoError(t, op.ProcessBatch(ctx, []*entry.Entry{e}))
			}

			fake.ExpectEntries(t, tc.expectedOutput)

			select {
			case e := <-fake.Received:
				require.FailNow(t, "Received unexpected entry: ", "%+v", e)
			default:
			}
		})
	}

	t.Run("FlushesOnShutdown", func(t *testing.T) {
		cfg := NewConfig()
		cfg.CombineField = entry.NewBodyField()
		cfg.IsFirstEntry = MatchAll
		cfg.OutputIDs = []string{"fake"}
		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
		require.NoError(t, err)

		fake := testutil.NewFakeOutput(t)
		err = op.SetOutputs([]operator.Operator{fake})
		require.NoError(t, err)

		// Send an entry that isn't the last in a multiline
		require.NoError(t, op.ProcessBatch(t.Context(), []*entry.Entry{entry.New()}))

		// Ensure that the entry isn't immediately sent
		select {
		case <-fake.Received:
			require.FailNow(t, "Received unexpected entry")
		case <-time.After(10 * time.Millisecond):
		}

		// Stop the operator
		require.NoError(t, op.Stop())

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
	cfg.SourceIdentifier = entry.NewAttributeField(attrs.LogFilePath)
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(b, err)

	fake := testutil.NewFakeOutput(b)
	require.NoError(b, op.SetOutputs([]operator.Operator{fake}))

	go func() {
		for range fake.Received { //nolint:revive
			// Nothing to do
		}
	}()

	sourcesNum := 10
	logsNum := 10
	entries := []*entry.Entry{}
	for i := range logsNum {
		for j := range sourcesNum {
			start := entry.New()
			start.Timestamp = time.Now()
			start.Body = strings.Repeat(fmt.Sprintf("log-%d", i), 50)
			start.Attributes = map[string]any{attrs.LogFilePath: fmt.Sprintf("file-%d", j)}
			entries = append(entries, start)
		}
	}

	ctx := b.Context()

	b.ReportAllocs()
	for b.Loop() {
		for _, e := range entries {
			require.NoError(b, op.ProcessBatch(b.Context(), []*entry.Entry{e}))
		}
		op.(*Transformer).flushAllSources(ctx, op.(*Transformer).Write)
	}
	b.StopTimer()

	require.NoError(b, op.Stop())
	close(fake.Received)
}

func BenchmarkRecombineLimitTrigger(b *testing.B) {
	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsFirstEntry = "body == 'start'"
	cfg.MaxLogSize = 6
	cfg.OutputIDs = []string{"fake"}
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(b, err)

	fake := testutil.NewFakeOutput(b)
	require.NoError(b, op.SetOutputs([]operator.Operator{fake}))
	require.NoError(b, op.Start(nil))

	go func() {
		for range fake.Received { //nolint:revive
			// Nothing to do
		}
	}()

	start := entry.New()
	start.Timestamp = time.Now()
	start.Body = "start"

	next := entry.New()
	next.Timestamp = time.Now()
	next.Body = "next"

	ctx := b.Context()

	for b.Loop() {
		require.NoError(b, op.ProcessBatch(ctx, []*entry.Entry{start, next}))
		require.NoError(b, op.ProcessBatch(ctx, []*entry.Entry{start, next}))
		op.(*Transformer).flushAllSources(ctx, op.(*Transformer).Write)
	}
	b.StopTimer()

	require.NoError(b, op.Stop())
	close(fake.Received)
}

func TestTimeout(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsFirstEntry = MatchAll
	cfg.OutputIDs = []string{"fake"}
	cfg.ForceFlushTimeout = 100 * time.Millisecond
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	recombine := op.(*Transformer)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, recombine.SetOutputs([]operator.Operator{fake}))

	e := entry.New()
	e.Timestamp = time.Now()
	e.Body = "body"

	ctx := t.Context()

	require.NoError(t, recombine.Start(nil))
	require.NoError(t, recombine.ProcessBatch(ctx, []*entry.Entry{e}))
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
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

	e := entry.New()
	e.Timestamp = time.Now()
	e.Body = "start"

	ctx := t.Context()

	require.NoError(t, op.Start(nil))
	require.NoError(t, op.ProcessBatch(ctx, []*entry.Entry{e}))

	done := make(chan struct{})
	ticker := time.NewTicker(cfg.ForceFlushTimeout / 2)
	go func() {
		next := entry.New()
		next.Timestamp = time.Now()
		next.Body = "next"
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				assert.NoError(t, op.ProcessBatch(ctx, []*entry.Entry{next}))
			}
		}
	}()

	select {
	case <-fake.Received:
	case <-time.After(5 * time.Second):
		t.Logf("The entry should be flushed by now")
		t.FailNow()
	}
	require.NoError(t, op.Stop())
	close(done)
}

func TestSourceBatchDelete(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsFirstEntry = "body == 'start'"
	cfg.OutputIDs = []string{"fake"}
	cfg.ForceFlushTimeout = 100 * time.Millisecond
	cfg.MaxLogSize = 6
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	recombine := op.(*Transformer)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, recombine.SetOutputs([]operator.Operator{fake}))

	start := entry.New()
	start.Timestamp = time.Now()
	start.Body = "start"
	start.AddAttribute(attrs.LogFilePath, "file1")

	next := entry.New()
	next.Timestamp = time.Now()
	next.Body = "next"
	next.AddAttribute(attrs.LogFilePath, "file1")

	expect := entry.New()
	expect.ObservedTimestamp = start.ObservedTimestamp
	expect.Timestamp = start.Timestamp
	expect.AddAttribute(attrs.LogFilePath, "file1")
	expect.Body = "start\nnext"

	ctx := t.Context()

	require.NoError(t, op.ProcessBatch(ctx, []*entry.Entry{start}))
	require.Len(t, recombine.batchMap, 1)
	require.NoError(t, op.ProcessBatch(ctx, []*entry.Entry{next}))
	require.Empty(t, recombine.batchMap)
	fake.ExpectEntry(t, expect)
	require.NoError(t, op.Stop())
}

func TestProcessBatchPreservesBatching(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsLastEntry = "body == 'END'"
	cfg.SourceIdentifier = entry.NewAttributeField(attrs.LogFilePath)
	cfg.OutputIDs = []string{"fake"}

	op, err := cfg.Build(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

	// Create entries from 3 different sources
	entry1_1 := entry.New()
	entry1_1.Body = "line1"
	entry1_1.AddAttribute(attrs.LogFilePath, "file1")

	entry1_2 := entry.New()
	entry1_2.Body = "END"
	entry1_2.AddAttribute(attrs.LogFilePath, "file1")

	entry2_1 := entry.New()
	entry2_1.Body = "line1"
	entry2_1.AddAttribute(attrs.LogFilePath, "file2")

	entry2_2 := entry.New()
	entry2_2.Body = "END"
	entry2_2.AddAttribute(attrs.LogFilePath, "file2")

	entry3_1 := entry.New()
	entry3_1.Body = "line1"
	entry3_1.AddAttribute(attrs.LogFilePath, "file3")

	entry3_2 := entry.New()
	entry3_2.Body = "END"
	entry3_2.AddAttribute(attrs.LogFilePath, "file3")

	// Process all entries in a single batch
	allEntries := []*entry.Entry{entry1_1, entry1_2, entry2_1, entry2_2, entry3_1, entry3_2}
	require.NoError(t, op.ProcessBatch(t.Context(), allEntries))

	// Verify we got 3 combined entries with correct content
	expect1 := entry.New()
	expect1.ObservedTimestamp = entry1_1.ObservedTimestamp
	expect1.Timestamp = entry1_1.Timestamp
	expect1.AddAttribute(attrs.LogFilePath, "file1")
	expect1.Body = "line1\nEND"

	expect2 := entry.New()
	expect2.ObservedTimestamp = entry2_1.ObservedTimestamp
	expect2.Timestamp = entry2_1.Timestamp
	expect2.AddAttribute(attrs.LogFilePath, "file2")
	expect2.Body = "line1\nEND"

	expect3 := entry.New()
	expect3.ObservedTimestamp = entry3_1.ObservedTimestamp
	expect3.Timestamp = entry3_1.Timestamp
	expect3.AddAttribute(attrs.LogFilePath, "file3")
	expect3.Body = "line1\nEND"

	fake.ExpectEntry(t, expect1)
	fake.ExpectEntry(t, expect2)
	fake.ExpectEntry(t, expect3)
}

func TestRecombineQuietModeProcess(t *testing.T) {
	testCases := []struct {
		name        string
		onError     string
		expectError bool
	}{
		{
			name:        "DropOnErrorQuiet_ReturnsNoError",
			onError:     helper.DropOnErrorQuiet,
			expectError: false,
		},
		{
			name:        "SendOnErrorQuiet_ReturnsNoError",
			onError:     helper.SendOnErrorQuiet,
			expectError: false,
		},
		{
			name:        "DropOnError_ReturnsError",
			onError:     helper.DropOnError,
			expectError: true,
		},
		{
			name:        "SendOnError_ReturnsError",
			onError:     helper.SendOnError,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.CombineField = entry.NewBodyField()
			// Use an invalid expression that will cause an error during processing
			cfg.IsLastEntry = "body.invalid_field == 'test'"
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = tc.onError

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))
			require.NoError(t, op.Start(nil))
			defer func() { require.NoError(t, op.Stop()) }()

			// Create entry that will cause expression evaluation error
			e := entry.New()
			e.Body = "test"
			e.ObservedTimestamp = time.Now()

			err = op.Process(t.Context(), e)
			if tc.expectError {
				require.Error(t, err, "expected error in non-quiet mode")
			} else {
				require.NoError(t, err, "expected no error in quiet mode")
			}
		})
	}
}

func TestRecombineQuietModeProcessBatch(t *testing.T) {
	testCases := []struct {
		name        string
		onError     string
		expectError bool
	}{
		{
			name:        "DropOnErrorQuiet_ReturnsNoError",
			onError:     helper.DropOnErrorQuiet,
			expectError: false,
		},
		{
			name:        "SendOnErrorQuiet_ReturnsNoError",
			onError:     helper.SendOnErrorQuiet,
			expectError: false,
		},
		{
			name:        "DropOnError_ReturnsError",
			onError:     helper.DropOnError,
			expectError: true,
		},
		{
			name:        "SendOnError_ReturnsError",
			onError:     helper.SendOnError,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.CombineField = entry.NewBodyField()
			// Use an invalid expression that will cause an error during processing
			cfg.IsLastEntry = "body.invalid_field == 'test'"
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = tc.onError

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))
			require.NoError(t, op.Start(nil))
			defer func() { require.NoError(t, op.Stop()) }()

			// Create entries that will cause expression evaluation errors
			entries := make([]*entry.Entry, 3)
			for i := range entries {
				e := entry.New()
				e.Body = "test"
				e.ObservedTimestamp = time.Now()
				entries[i] = e
			}

			err = op.ProcessBatch(t.Context(), entries)
			if tc.expectError {
				require.Error(t, err, "expected error in non-quiet mode")
			} else {
				require.NoError(t, err, "expected no error in quiet mode")
			}
		})
	}
}

// TestRecombineFlushSource tests that flushSource errors are always returned
// regardless of quiet mode setting. Pipeline write errors should never be suppressed as they
// indicate systemic issues and silent data loss.
func TestRecombineFlushSource(t *testing.T) {
	testCases := []struct {
		name    string
		onError string
	}{
		{
			name:    "DropOnErrorQuiet",
			onError: helper.DropOnErrorQuiet,
		},
		{
			name:    "SendOnErrorQuiet",
			onError: helper.SendOnErrorQuiet,
		},
		{
			name:    "DropOnError",
			onError: helper.DropOnError,
		},
		{
			name:    "SendOnError",
			onError: helper.SendOnError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name+"_Process_MatchFirstLine", func(t *testing.T) {
			cfg := NewConfig()
			cfg.CombineField = entry.NewBodyField()
			cfg.IsFirstEntry = "body == 'START'"
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = tc.onError

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			// Use an output that will fail on write to trigger flushSource error
			failingOutput := &testutil.Operator{}
			failingOutput.On("ID").Return("fake")
			failingOutput.On("CanProcess").Return(true)
			failingOutput.On("Process", mock.Anything, mock.Anything).Return(errors.New("write error"))
			require.NoError(t, op.SetOutputs([]operator.Operator{failingOutput}))
			require.NoError(t, op.Start(nil))
			defer func() { require.NoError(t, op.Stop()) }()

			// First entry - starts batch
			e1 := entry.New()
			e1.Body = "START"
			e1.ObservedTimestamp = time.Now()
			err = op.Process(t.Context(), e1)
			require.NoError(t, err) // First entry just starts batch, no flush yet

			// Second entry with START triggers flush of first batch
			e2 := entry.New()
			e2.Body = "START"
			e2.ObservedTimestamp = time.Now()
			err = op.Process(t.Context(), e2)
			// flushSource errors should always be returned regardless of quiet mode
			require.Error(t, err, "flushSource errors should always be returned")
		})

		t.Run(tc.name+"_Process_MatchLastLine", func(t *testing.T) {
			cfg := NewConfig()
			cfg.CombineField = entry.NewBodyField()
			cfg.IsLastEntry = "body == 'END'"
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = tc.onError

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			// Use an output that will fail on write to trigger flushSource error
			failingOutput := &testutil.Operator{}
			failingOutput.On("ID").Return("fake")
			failingOutput.On("CanProcess").Return(true)
			failingOutput.On("Process", mock.Anything, mock.Anything).Return(errors.New("write error"))
			require.NoError(t, op.SetOutputs([]operator.Operator{failingOutput}))
			require.NoError(t, op.Start(nil))
			defer func() { require.NoError(t, op.Stop()) }()

			// First entry - starts batch
			e1 := entry.New()
			e1.Body = "line1"
			e1.ObservedTimestamp = time.Now()
			err = op.Process(t.Context(), e1)
			require.NoError(t, err) // First entry just starts batch, no flush yet

			// Second entry with END triggers flush
			e2 := entry.New()
			e2.Body = "END"
			e2.ObservedTimestamp = time.Now()
			err = op.Process(t.Context(), e2)
			// flushSource errors should always be returned regardless of quiet mode
			require.Error(t, err, "flushSource errors should always be returned")
		})

		t.Run(tc.name+"_ProcessBatch_MatchFirstLine", func(t *testing.T) {
			cfg := NewConfig()
			cfg.CombineField = entry.NewBodyField()
			cfg.IsFirstEntry = "body == 'START'"
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = tc.onError

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			// Use an output that will fail on write to trigger flushSource error
			failingOutput := &testutil.Operator{}
			failingOutput.On("ID").Return("fake")
			failingOutput.On("CanProcess").Return(true)
			failingOutput.On("Process", mock.Anything, mock.Anything).Return(errors.New("write error"))
			failingOutput.On("ProcessBatch", mock.Anything, mock.Anything).Return(errors.New("write error"))
			require.NoError(t, op.SetOutputs([]operator.Operator{failingOutput}))
			require.NoError(t, op.Start(nil))
			defer func() { require.NoError(t, op.Stop()) }()

			// Create batch: first START starts batch, second START triggers flush
			entries := []*entry.Entry{
				func() *entry.Entry {
					e := entry.New()
					e.Body = "START"
					e.ObservedTimestamp = time.Now()
					return e
				}(),
				func() *entry.Entry {
					e := entry.New()
					e.Body = "START"
					e.ObservedTimestamp = time.Now()
					return e
				}(),
			}

			err = op.ProcessBatch(t.Context(), entries)
			// flushSource errors should always be returned regardless of quiet mode
			require.Error(t, err, "flushSource errors should always be returned")
		})

		t.Run(tc.name+"_ProcessBatch_MatchLastLine", func(t *testing.T) {
			cfg := NewConfig()
			cfg.CombineField = entry.NewBodyField()
			cfg.IsLastEntry = "body == 'END'"
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = tc.onError

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			// Use an output that will fail on write to trigger flushSource error
			failingOutput := &testutil.Operator{}
			failingOutput.On("ID").Return("fake")
			failingOutput.On("CanProcess").Return(true)
			failingOutput.On("Process", mock.Anything, mock.Anything).Return(errors.New("write error"))
			failingOutput.On("ProcessBatch", mock.Anything, mock.Anything).Return(errors.New("write error"))
			require.NoError(t, op.SetOutputs([]operator.Operator{failingOutput}))
			require.NoError(t, op.Start(nil))
			defer func() { require.NoError(t, op.Stop()) }()

			// Create batch: first entry starts batch, END triggers flush
			entries := []*entry.Entry{
				func() *entry.Entry {
					e := entry.New()
					e.Body = "line1"
					e.ObservedTimestamp = time.Now()
					return e
				}(),
				func() *entry.Entry {
					e := entry.New()
					e.Body = "END"
					e.ObservedTimestamp = time.Now()
					return e
				}(),
			}

			err = op.ProcessBatch(t.Context(), entries)
			// flushSource errors should always be returned regardless of quiet mode
			require.Error(t, err, "flushSource errors should always be returned")
		})
	}
}
