// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regex

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T, regex string, cacheSize uint16) *Parser {
	cfg := NewConfigWithID("test")
	cfg.Regex = regex
	if cacheSize > 0 {
		cfg.Cache.Size = cacheSize
	}
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*Parser)
}

func TestParserBuildFailure(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.OnError = "invalid_on_error"
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestParserByteFailure(t *testing.T) {
	parser := newTestParser(t, "^(?P<key>test)", 0)
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]uint8' cannot be parsed as regex")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t, "^(?P<key>test)", 0)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "regex pattern does not match")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t, "^(?P<key>test)", 0)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as regex")
}

func TestParserCache(t *testing.T) {
	parser := newTestParser(t, "^(?P<key>cache)", 200)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as regex")
	require.NotNil(t, parser.cache, "expected cache to be configured")
	require.Equal(t, parser.cache.maxSize(), uint16(200))
}

func TestParserRegex(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*Config)
		input     *entry.Entry
		expected  *entry.Entry
	}{
		{
			"RootString",
			func(p *Config) {
				p.Regex = "a=(?P<a>.*)"
			},
			&entry.Entry{
				Body: "a=b",
			},
			&entry.Entry{
				Body: "a=b",
				Attributes: map[string]any{
					"a": "b",
				},
			},
		},
		{
			"MemeoryCache",
			func(p *Config) {
				p.Regex = "a=(?P<a>.*)"
				p.Cache.Size = 100
			},
			&entry.Entry{
				Body: "a=b",
			},
			&entry.Entry{
				Body: "a=b",
				Attributes: map[string]any{
					"a": "b",
				},
			},
		},
		{
			"K8sFileCache",
			func(p *Config) {
				p.Regex = `^(?P<pod_name>[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)_(?P<namespace>[^_]+)_(?P<container_name>.+)-(?P<container_id>[a-z0-9]{64})\.log$`
				p.Cache.Size = 100
			},
			&entry.Entry{
				Body: "coredns-5644d7b6d9-mzngq_kube-system_coredns-901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6.log",
			},
			&entry.Entry{
				Body: "coredns-5644d7b6d9-mzngq_kube-system_coredns-901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6.log",
				Attributes: map[string]any{
					"container_id":   "901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6",
					"container_name": "coredns",
					"namespace":      "kube-system",
					"pod_name":       "coredns-5644d7b6d9-mzngq",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

			ots := time.Now()
			tc.input.ObservedTimestamp = ots
			tc.expected.ObservedTimestamp = ots

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)

			fake.ExpectEntry(t, tc.expected)
		})
	}
}

func TestBuildParserRegex(t *testing.T) {
	newBasicParser := func() *Config {
		cfg := NewConfigWithID("test")
		cfg.OutputIDs = []string{"test"}
		cfg.Regex = "(?P<all>.*)"
		return cfg
	}

	t.Run("BasicConfig", func(t *testing.T) {
		c := newBasicParser()
		_, err := c.Build(testutil.Logger(t))
		require.NoError(t, err)
	})

	t.Run("MissingRegexField", func(t *testing.T) {
		c := newBasicParser()
		c.Regex = ""
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
	})

	t.Run("InvalidRegexField", func(t *testing.T) {
		c := newBasicParser()
		c.Regex = "())()"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
	})

	t.Run("NoNamedGroups", func(t *testing.T) {
		c := newBasicParser()
		c.Regex = ".*"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "no named capture groups")
	})

	t.Run("NoNamedGroups", func(t *testing.T) {
		c := newBasicParser()
		c.Regex = "(.*)"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "no named capture groups")
	})
}

// return 100 unique file names, example:
// dafplsjfbcxoeff-5644d7b6d9-mzngq_kube-system_coredns-901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6.log
// rswxpldnjobcsnv-5644d7b6d9-mzngq_kube-system_coredns-901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6.log
// lgtemapezqleqyh-5644d7b6d9-mzngq_kube-system_coredns-901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6.log
func benchParseInput() (patterns []string) {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz"
	for i := 1; i <= 100; i++ {
		b := make([]byte, 15)
		for i := range b {
			b[i] = letterBytes[rand.Intn(len(letterBytes))]
		}
		randomStr := string(b)
		p := fmt.Sprintf("%s-5644d7b6d9-mzngq_kube-system_coredns-901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6.log", randomStr)
		patterns = append(patterns, p)
	}
	return patterns
}

// Regex used to parse a kubernetes container log file name, which contains the
// pod name, namespace, container name, container.
const benchParsePattern = `^(?P<pod_name>[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)_(?P<namespace>[^_]+)_(?P<container_name>.+)-(?P<container_id>[a-z0-9]{64})\.log$`

var benchParsePatterns = benchParseInput()

func newTestBenchParser(t *testing.T, cacheSize uint16) *Parser {
	cfg := NewConfigWithID("bench")
	cfg.Regex = benchParsePattern
	cfg.Cache.Size = cacheSize

	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*Parser)
}

func benchmarkParseThreaded(b *testing.B, parser *Parser, input []string) {
	wg := sync.WaitGroup{}

	for _, i := range input {
		wg.Add(1)

		go func(i string) {
			if _, err := parser.match(i); err != nil {
				b.Error(err)
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func benchmarkParse(b *testing.B, parser *Parser, input []string) {
	for _, i := range input {
		if _, err := parser.match(i); err != nil {
			b.Error(err)
		}
	}
}

// No cache
func BenchmarkParseNoCache(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 0)
	for n := 0; n < b.N; n++ {
		benchmarkParseThreaded(b, parser, benchParsePatterns)
	}
}

// Memory cache at capacity
func BenchmarkParseWithMemoryCache(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 100)
	for n := 0; n < b.N; n++ {
		benchmarkParseThreaded(b, parser, benchParsePatterns)
	}
}

// Memory cache over capacity by one
func BenchmarkParseWithMemoryCacheFullByOne(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 99)
	for n := 0; n < b.N; n++ {
		benchmarkParseThreaded(b, parser, benchParsePatterns)
	}
}

// Memory cache over capacity by 10
func BenchmarkParseWithMemoryCacheFullBy10(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 90)
	for n := 0; n < b.N; n++ {
		benchmarkParseThreaded(b, parser, benchParsePatterns)
	}
}

// Memory cache over capacity by 50
func BenchmarkParseWithMemoryCacheFullBy50(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 50)
	for n := 0; n < b.N; n++ {
		benchmarkParseThreaded(b, parser, benchParsePatterns)
	}
}

// Memory cache over capacity by 90
func BenchmarkParseWithMemoryCacheFullBy90(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 10)
	for n := 0; n < b.N; n++ {
		benchmarkParseThreaded(b, parser, benchParsePatterns)
	}
}

// Memory cache over capacity by 99
func BenchmarkParseWithMemoryCacheFullBy99(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 1)
	for n := 0; n < b.N; n++ {
		benchmarkParseThreaded(b, parser, benchParsePatterns)
	}
}

// No cache one file
func BenchmarkParseNoCacheOneFile(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 0)
	for n := 0; n < b.N; n++ {
		pattern := []string{benchParsePatterns[0]}
		benchmarkParse(b, parser, pattern)
	}
}

// Memory cache one file
func BenchmarkParseWithMemoryCacheOneFile(b *testing.B) {
	parser := newTestBenchParser(&testing.T{}, 100)
	for n := 0; n < b.N; n++ {
		pattern := []string{benchParsePatterns[0]}
		benchmarkParse(b, parser, pattern)
	}
}
