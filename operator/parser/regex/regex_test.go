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

package regex

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func newTestParser(t *testing.T, regex string, cacheSize uint16) *RegexParser {
	cfg := NewRegexParserConfig("test")
	cfg.Regex = regex
	if cacheSize > 0 {
		cfg.Cache.Size = cacheSize
	}
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*RegexParser)
}

func TestRegexParserBuildFailure(t *testing.T) {
	cfg := NewRegexParserConfig("test")
	cfg.OnError = "invalid_on_error"
	_, err := cfg.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestRegexParserByteFailure(t *testing.T) {
	parser := newTestParser(t, "^(?P<key>test)", 0)
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]uint8' cannot be parsed as regex")
}

func TestRegexParserStringFailure(t *testing.T) {
	parser := newTestParser(t, "^(?P<key>test)", 0)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "regex pattern does not match")
}

func TestRegexParserInvalidType(t *testing.T) {
	parser := newTestParser(t, "^(?P<key>test)", 0)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type '[]int' cannot be parsed as regex")
}

func TestRegexParserCache(t *testing.T) {
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
		configure func(*RegexParserConfig)
		input     *entry.Entry
		expected  *entry.Entry
	}{
		{
			"RootString",
			func(p *RegexParserConfig) {
				p.Regex = "a=(?P<a>.*)"
			},
			&entry.Entry{
				Body: "a=b",
			},
			&entry.Entry{
				Body: "a=b",
				Attributes: map[string]interface{}{
					"a": "b",
				},
			},
		},
		{
			"MemeoryCache",
			func(p *RegexParserConfig) {
				p.Regex = "a=(?P<a>.*)"
				p.Cache.Size = 100
			},
			&entry.Entry{
				Body: "a=b",
			},
			&entry.Entry{
				Body: "a=b",
				Attributes: map[string]interface{}{
					"a": "b",
				},
			},
		},
		{
			"K8sFileCache",
			func(p *RegexParserConfig) {
				p.Regex = `^(?P<pod_name>[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)_(?P<namespace>[^_]+)_(?P<container_name>.+)-(?P<container_id>[a-z0-9]{64})\.log$`
				p.Cache.Size = 100
			},
			&entry.Entry{
				Body: "coredns-5644d7b6d9-mzngq_kube-system_coredns-901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6.log",
			},
			&entry.Entry{
				Body: "coredns-5644d7b6d9-mzngq_kube-system_coredns-901f7510281180a402936c92f5bc0f3557f5a21ccb5a4591c5bf98f3ddbffdd6.log",
				Attributes: map[string]interface{}{
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
			cfg := NewRegexParserConfig("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			op.SetOutputs([]operator.Operator{fake})

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
	newBasicRegexParser := func() *RegexParserConfig {
		cfg := NewRegexParserConfig("test")
		cfg.OutputIDs = []string{"test"}
		cfg.Regex = "(?P<all>.*)"
		return cfg
	}

	t.Run("BasicConfig", func(t *testing.T) {
		c := newBasicRegexParser()
		_, err := c.Build(testutil.Logger(t))
		require.NoError(t, err)
	})

	t.Run("MissingRegexField", func(t *testing.T) {
		c := newBasicRegexParser()
		c.Regex = ""
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
	})

	t.Run("InvalidRegexField", func(t *testing.T) {
		c := newBasicRegexParser()
		c.Regex = "())()"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
	})

	t.Run("NoNamedGroups", func(t *testing.T) {
		c := newBasicRegexParser()
		c.Regex = ".*"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "no named capture groups")
	})

	t.Run("NoNamedGroups", func(t *testing.T) {
		c := newBasicRegexParser()
		c.Regex = "(.*)"
		_, err := c.Build(testutil.Logger(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "no named capture groups")
	})
}

func TestRegexParserConfig(t *testing.T) {
	expect := NewRegexParserConfig("test")
	expect.Regex = "test123"
	expect.ParseFrom = entry.NewBodyField("from")
	expect.ParseTo = entry.NewBodyField("to")

	t.Run("mapstructure", func(t *testing.T) {
		input := map[string]interface{}{
			"id":         "test",
			"type":       "regex_parser",
			"regex":      "test123",
			"parse_from": "body.from",
			"parse_to":   "body.to",
			"on_error":   "send",
		}
		var actual RegexParserConfig
		err := helper.UnmarshalMapstructure(input, &actual)
		require.NoError(t, err)
		require.Equal(t, expect, &actual)
	})

	t.Run("yaml", func(t *testing.T) {
		input := `
type: regex_parser
id: test
on_error: "send"
regex: "test123"
parse_from: body.from
parse_to: body.to`
		var actual RegexParserConfig
		err := yaml.Unmarshal([]byte(input), &actual)
		require.NoError(t, err)
		require.Equal(t, expect, &actual)
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

func newTestBenchParser(t *testing.T, cacheSize uint16) *RegexParser {
	cfg := NewRegexParserConfig("bench")
	cfg.Regex = benchParsePattern
	cfg.Cache.Size = cacheSize

	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*RegexParser)
}

func benchmarkParseThreaded(b *testing.B, parser *RegexParser, input []string) {
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

func benchmarkParse(b *testing.B, parser *RegexParser, input []string) {
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
