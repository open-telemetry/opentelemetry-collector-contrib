// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package keyvalue

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T) *KVParser {
	config := NewKVParserConfig("test")
	op, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*KVParser)
}

func TestInit(t *testing.T) {
	builder, ok := operator.DefaultRegistry.Lookup("key_value_parser")
	require.True(t, ok, "expected key_value_parser to be registered")
	require.Equal(t, "key_value_parser", builder().Type())
}

func TestKVParserConfigBuild(t *testing.T) {
	config := NewKVParserConfig("test")
	op, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.IsType(t, &KVParser{}, op)
}

func TestKVParserConfigBuildFailure(t *testing.T) {
	config := NewKVParserConfig("test")
	config.OnError = "invalid_on_error"
	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestBuild(t *testing.T) {
	basicConfig := func() *KVParserConfig {
		cfg := NewKVParserConfig("test_operator_id")
		return cfg
	}

	cases := []struct {
		name      string
		input     *KVParserConfig
		expectErr bool
	}{
		{
			"default",
			func() *KVParserConfig {
				cfg := basicConfig()
				return cfg
			}(),
			false,
		},
		{
			"delimiter",
			func() *KVParserConfig {
				cfg := basicConfig()
				cfg.Delimiter = "/"
				return cfg
			}(),
			false,
		},
		{
			"missing-delimiter",
			func() *KVParserConfig {
				cfg := basicConfig()
				cfg.Delimiter = ""
				return cfg
			}(),
			true,
		},
		{
			"pair-delimiter",
			func() *KVParserConfig {
				cfg := basicConfig()
				cfg.PairDelimiter = "|"
				return cfg
			}(),
			false,
		},
		{
			"same-delimiter-and-pair-delimiter",
			func() *KVParserConfig {
				cfg := basicConfig()
				cfg.Delimiter = "|"
				cfg.PairDelimiter = cfg.Delimiter
				return cfg
			}(),
			true,
		},
		{
			"unset-delimiter",
			func() *KVParserConfig {
				cfg := basicConfig()
				cfg.Delimiter = ""
				cfg.PairDelimiter = "!"
				return cfg
			}(),
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.input
			_, err := cfg.Build(testutil.Logger(t))
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestKVParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("expected '%s' to split by '%s' into two items, got", "invalid", parser.delimiter))
}

func TestKVParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []int cannot be parsed as key value pairs")
}

func TestKVImplementations(t *testing.T) {
	require.Implements(t, (*operator.Operator)(nil), new(KVParser))
}

func TestKVParser(t *testing.T) {
	cases := []struct {
		name           string
		configure      func(*KVParserConfig)
		input          *entry.Entry
		expect         *entry.Entry
		expectError    bool
		expectBuildErr bool
	}{
		{
			"simple",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: "name=stanza age=2",
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"name": "stanza",
					"age":  "2",
				},
				Body: "name=stanza age=2",
			},
			false,
			false,
		},
		{
			"parse-from",
			func(kv *KVParserConfig) {
				kv.ParseFrom = entry.NewBodyField("test")
			},
			&entry.Entry{
				Body: map[string]interface{}{
					"test": "name=otel age=2",
				},
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"name": "otel",
					"age":  "2",
				},
				Body: map[string]interface{}{
					"test": "name=otel age=2",
				},
			},
			false,
			false,
		},
		{
			"parse-to",
			func(kv *KVParserConfig) {
				kv.ParseTo = entry.NewBodyField("test")
			},
			&entry.Entry{
				Body: "name=stanza age=10",
			},
			&entry.Entry{
				Body: map[string]interface{}{
					"test": map[string]interface{}{
						"name": "stanza",
						"age":  "10",
					},
				},
			},
			false,
			false,
		},
		{
			"from-to",
			func(kv *KVParserConfig) {
				kv.ParseFrom = entry.NewAttributeField("from")
				kv.ParseTo = entry.NewBodyField("to")
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"from": "name=stanza age=10",
				},
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"from": "name=stanza age=10",
				},
				Body: map[string]interface{}{
					"to": map[string]interface{}{
						"name": "stanza",
						"age":  "10",
					},
				},
			},
			false,
			false,
		},
		{
			"user-agent",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: `requestClientApplication="Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0"`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"requestClientApplication": `Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0`,
				},
				Body: `requestClientApplication="Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0"`,
			},
			false,
			false,
		},
		{
			"double-quotes-removed",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: "name=\"stanza\" age=2",
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"name": "stanza",
					"age":  "2",
				},
				Body: "name=\"stanza\" age=2",
			},
			false,
			false,
		},
		{
			"single-quotes-removed",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: "description='stanza deployment number 5' x=y",
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"description": "stanza deployment number 5",
					"x":           "y",
				},
				Body: "description='stanza deployment number 5' x=y",
			},
			false,
			false,
		},
		{
			"double-quotes-spaces-removed",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: `name=" stanza " age=2`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"name": "stanza",
					"age":  "2",
				},
				Body: `name=" stanza " age=2`,
			},
			false,
			false,
		},
		{
			"leading-and-trailing-space",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: `" name "=" stanza " age=2`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"name": "stanza",
					"age":  "2",
				},
				Body: `" name "=" stanza " age=2`,
			},
			false,
			false,
		},
		{
			"delimiter",
			func(kv *KVParserConfig) {
				kv.Delimiter = "|"
				kv.ParseFrom = entry.NewBodyField("testfield")
				kv.ParseTo = entry.NewBodyField("testparsed")
			},
			&entry.Entry{
				Body: map[string]interface{}{
					"testfield": `name|" stanza " age|2     key|value`,
				},
			},
			&entry.Entry{
				Body: map[string]interface{}{
					"testfield": `name|" stanza " age|2     key|value`,
					"testparsed": map[string]interface{}{
						"name": "stanza",
						"age":  "2",
						"key":  "value",
					},
				},
			},
			false,
			false,
		},
		{
			"double-delimiter",
			func(kv *KVParserConfig) {
				kv.Delimiter = "=="
			},
			&entry.Entry{
				Body: `name==" stanza " age==2     key==value`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"name": "stanza",
					"age":  "2",
					"key":  "value",
				},
				Body: `name==" stanza " age==2     key==value`,
			},
			false,
			false,
		},
		{
			"pair-delimiter",
			func(kv *KVParserConfig) {
				kv.PairDelimiter = "|"
			},
			&entry.Entry{
				Body: `name=stanza|age=2     | key=value`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"name": "stanza",
					"age":  "2",
					"key":  "value",
				},
				Body: `name=stanza|age=2     | key=value`,
			},
			false,
			false,
		},
		{
			"large",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: "name=stanza age=1 job=\"software engineering\" location=\"grand rapids michigan\" src=\"10.3.3.76\" dst=172.217.0.10 protocol=udp sport=57112 dport=443 translated_src_ip=96.63.176.3 translated_port=57112",
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"age":               "1",
					"dport":             "443",
					"dst":               "172.217.0.10",
					"job":               "software engineering",
					"location":          "grand rapids michigan",
					"name":              "stanza",
					"protocol":          "udp",
					"sport":             "57112",
					"src":               "10.3.3.76",
					"translated_port":   "57112",
					"translated_src_ip": "96.63.176.3",
				},
				Body: "name=stanza age=1 job=\"software engineering\" location=\"grand rapids michigan\" src=\"10.3.3.76\" dst=172.217.0.10 protocol=udp sport=57112 dport=443 translated_src_ip=96.63.176.3 translated_port=57112",
			},
			false,
			false,
		},
		{
			"dell-sonic-wall",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: `id=LVM_Sonicwall sn=22255555 time="2021-09-22 16:30:31" fw=14.165.177.10 pri=6 c=1024 gcat=2 m=97 msg="Web site hit" srcMac=6c:0b:84:3f:fa:63 src=192.168.50.2:52006:X0 srcZone=LAN natSrc=14.165.177.10:58457 dstMac=08:b2:58:46:30:54 dst=15.159.150.83:443:X1 dstZone=WAN natDst=15.159.150.83:443 proto=tcp/https sent=1422 rcvd=5993 rule="6 (LAN->WAN)" app=48 dstname=example.space.dev.com arg=/ code=27 Category="Information Technology/Computers" note="Policy: a0, Info: 888 " n=3412158`,
			},
			&entry.Entry{
				Attributes: map[string]interface{}{
					"id":       "LVM_Sonicwall",
					"sn":       "22255555",
					"time":     "2021-09-22 16:30:31",
					"fw":       "14.165.177.10",
					"pri":      "6",
					"c":        "1024",
					"gcat":     "2",
					"m":        "97",
					"msg":      "Web site hit",
					"srcMac":   "6c:0b:84:3f:fa:63",
					"src":      "192.168.50.2:52006:X0",
					"srcZone":  "LAN",
					"natSrc":   "14.165.177.10:58457",
					"dstMac":   "08:b2:58:46:30:54",
					"dst":      "15.159.150.83:443:X1",
					"dstZone":  "WAN",
					"natDst":   "15.159.150.83:443",
					"proto":    "tcp/https",
					"sent":     "1422",
					"rcvd":     "5993",
					"rule":     "6 (LAN->WAN)",
					"app":      "48",
					"dstname":  "example.space.dev.com",
					"arg":      "/",
					"code":     "27",
					"Category": "Information Technology/Computers",
					"note":     "Policy: a0, Info: 888",
					"n":        "3412158",
				},
				Body: `id=LVM_Sonicwall sn=22255555 time="2021-09-22 16:30:31" fw=14.165.177.10 pri=6 c=1024 gcat=2 m=97 msg="Web site hit" srcMac=6c:0b:84:3f:fa:63 src=192.168.50.2:52006:X0 srcZone=LAN natSrc=14.165.177.10:58457 dstMac=08:b2:58:46:30:54 dst=15.159.150.83:443:X1 dstZone=WAN natDst=15.159.150.83:443 proto=tcp/https sent=1422 rcvd=5993 rule="6 (LAN->WAN)" app=48 dstname=example.space.dev.com arg=/ code=27 Category="Information Technology/Computers" note="Policy: a0, Info: 888 " n=3412158`,
			},
			false,
			false,
		},
		{
			"missing-delimiter",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: `test text`,
			},
			&entry.Entry{
				Body: `test text`,
			},
			true,
			false,
		},
		{
			"invalid-pair",
			func(kv *KVParserConfig) {},
			&entry.Entry{
				Body: `test=text=abc`,
			},
			&entry.Entry{
				Body: `test=text=abc`,
			},
			true,
			false,
		},
		{
			"empty-input",
			func(kv *KVParserConfig) {},
			&entry.Entry{},
			&entry.Entry{},
			true,
			false,
		},
		{
			"same-delimiter-and-pair-delimiter",
			func(kv *KVParserConfig) {
				kv.Delimiter = "!"
				kv.PairDelimiter = kv.Delimiter
			},
			&entry.Entry{
				Body: "a=b c=d",
			},
			&entry.Entry{
				Body: "a=b c=d",
			},
			false,
			true,
		},
		{
			"unset-delimiter",
			func(kv *KVParserConfig) {
				kv.Delimiter = ""
				kv.PairDelimiter = "!"
			},
			&entry.Entry{
				Body: "a=b c=d",
			},
			&entry.Entry{
				Body: "a=b c=d",
			},
			false,
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewKVParserConfig("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			op, err := cfg.Build(testutil.Logger(t))
			if tc.expectBuildErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

			ots := time.Now()
			tc.input.ObservedTimestamp = ots
			tc.expect.ObservedTimestamp = ots

			err = op.Process(context.Background(), tc.input)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			fake.ExpectEntry(t, tc.expect)
		})
	}
}

func TestSplitStringByWhitespace(t *testing.T) {
	cases := []struct {
		name   string
		intput string
		output []string
	}{
		{
			"simple",
			"k=v a=b x=\" y \" job=\"software engineering\"",
			[]string{
				"k=v",
				"a=b",
				"x=\" y \"",
				"job=\"software engineering\"",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.output, splitStringByWhitespace(tc.intput))
		})
	}
}
