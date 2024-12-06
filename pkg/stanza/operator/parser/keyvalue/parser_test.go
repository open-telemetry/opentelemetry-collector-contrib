// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package keyvalue

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T) *Parser {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	return op.(*Parser)
}

func TestInit(t *testing.T) {
	builder, ok := operator.DefaultRegistry.Lookup("key_value_parser")
	require.True(t, ok, "expected key_value_parser to be registered")
	require.Equal(t, "key_value_parser", builder().Type())
}

func TestConfigBuild(t *testing.T) {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	require.IsType(t, &Parser{}, op)
}

func TestConfigBuildFailure(t *testing.T) {
	config := NewConfigWithID("test")
	config.OnError = "invalid_on_error"
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.ErrorContains(t, err, "invalid `on_error` field")
}

func TestBuild(t *testing.T) {
	basicConfig := func() *Config {
		cfg := NewConfigWithID("test_operator_id")
		return cfg
	}

	cases := []struct {
		name      string
		input     *Config
		expectErr bool
	}{
		{
			"default",
			func() *Config {
				cfg := basicConfig()
				return cfg
			}(),
			false,
		},
		{
			"delimiter",
			func() *Config {
				cfg := basicConfig()
				cfg.Delimiter = "/"
				return cfg
			}(),
			false,
		},
		{
			"missing-delimiter",
			func() *Config {
				cfg := basicConfig()
				cfg.Delimiter = ""
				return cfg
			}(),
			true,
		},
		{
			"pair-delimiter",
			func() *Config {
				cfg := basicConfig()
				cfg.PairDelimiter = "|"
				return cfg
			}(),
			false,
		},
		{
			"pair-delimiter-multiline",
			func() *Config {
				cfg := basicConfig()
				cfg.PairDelimiter = "^\n"
				return cfg
			}(),
			false,
		},
		{
			"same-delimiter-and-pair-delimiter",
			func() *Config {
				cfg := basicConfig()
				cfg.Delimiter = "|"
				cfg.PairDelimiter = cfg.Delimiter
				return cfg
			}(),
			true,
		},
		{
			"pair-delimiter-equals-default-delimiter",
			func() *Config {
				cfg := basicConfig()
				cfg.Delimiter = " "
				return cfg
			}(),
			true,
		},
		{
			"unset-delimiter",
			func() *Config {
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
			set := componenttest.NewNopTelemetrySettings()
			_, err := cfg.Build(set)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.ErrorContains(t, err, fmt.Sprintf("cannot split %q into 2 items, got 1 item(s)", "invalid"))
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.ErrorContains(t, err, "type []int cannot be parsed as key value pairs")
}

func TestParserEmptyInput(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("")
	require.ErrorContains(t, err, "parse from field body is empty")
}

func TestKVImplementations(t *testing.T) {
	require.Implements(t, (*operator.Operator)(nil), new(Parser))
}

func TestParser(t *testing.T) {
	cases := []struct {
		name           string
		configure      func(*Config)
		input          *entry.Entry
		expect         *entry.Entry
		expectError    bool
		expectBuildErr bool
	}{
		{
			"simple",
			func(_ *Config) {},
			&entry.Entry{
				Body: "name=stanza age=2",
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			func(kv *Config) {
				kv.ParseFrom = entry.NewBodyField("test")
			},
			&entry.Entry{
				Body: map[string]any{
					"test": "name=otel age=2",
				},
			},
			&entry.Entry{
				Attributes: map[string]any{
					"name": "otel",
					"age":  "2",
				},
				Body: map[string]any{
					"test": "name=otel age=2",
				},
			},
			false,
			false,
		},
		{
			"parse-to",
			func(kv *Config) {
				kv.ParseTo = entry.RootableField{Field: entry.NewBodyField("test")}
			},
			&entry.Entry{
				Body: "name=stanza age=10",
			},
			&entry.Entry{
				Body: map[string]any{
					"test": map[string]any{
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
			func(kv *Config) {
				kv.ParseFrom = entry.NewAttributeField("from")
				kv.ParseTo = entry.RootableField{Field: entry.NewBodyField("to")}
			},
			&entry.Entry{
				Attributes: map[string]any{
					"from": "name=stanza age=10",
				},
			},
			&entry.Entry{
				Attributes: map[string]any{
					"from": "name=stanza age=10",
				},
				Body: map[string]any{
					"to": map[string]any{
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
			func(_ *Config) {},
			&entry.Entry{
				Body: `requestClientApplication="Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0"`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"requestClientApplication": `Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0`,
				},
				Body: `requestClientApplication="Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0"`,
			},
			false,
			false,
		},
		{
			"double-quotes-removed",
			func(_ *Config) {},
			&entry.Entry{
				Body: "name=\"stanza\" age=2",
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			func(_ *Config) {},
			&entry.Entry{
				Body: "description='stanza deployment number 5' x=y",
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			func(_ *Config) {},
			&entry.Entry{
				Body: `name=" stanza " age=2`,
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			func(_ *Config) {},
			&entry.Entry{
				Body: `" name "=" stanza " age=2`,
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			func(kv *Config) {
				kv.Delimiter = "|"
				kv.ParseFrom = entry.NewBodyField("testfield")
				kv.ParseTo = entry.RootableField{Field: entry.NewBodyField("testparsed")}
			},
			&entry.Entry{
				Body: map[string]any{
					"testfield": `name|" stanza " age|2     key|value`,
				},
			},
			&entry.Entry{
				Body: map[string]any{
					"testfield": `name|" stanza " age|2     key|value`,
					"testparsed": map[string]any{
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
			func(kv *Config) {
				kv.Delimiter = "=="
			},
			&entry.Entry{
				Body: `name==" stanza " age==2     key==value`,
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			func(kv *Config) {
				kv.PairDelimiter = "|"
			},
			&entry.Entry{
				Body: `name=stanza|age=2     | key=value`,
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			"pair-delimiter-multiline",
			func(kv *Config) {
				kv.PairDelimiter = "^\n"
			},
			&entry.Entry{
				Body: `name=stanza^
age=2^
key=value`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"name": "stanza",
					"age":  "2",
					"key":  "value",
				},
				Body: `name=stanza^
age=2^
key=value`,
			},
			false,
			false,
		},
		{
			"large",
			func(_ *Config) {},
			&entry.Entry{
				Body: "name=stanza age=1 job=\"software engineering\" location=\"grand rapids michigan\" src=\"10.3.3.76\" dst=172.217.0.10 protocol=udp sport=57112 dport=443 translated_src_ip=96.63.176.3 translated_port=57112",
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			func(_ *Config) {},
			&entry.Entry{
				Body: `id=LVM_Sonicwall sn=22255555 time="2021-09-22 16:30:31" fw=14.165.177.10 pri=6 c=1024 gcat=2 m=97 msg="Web site hit" srcMac=6c:0b:84:3f:fa:63 src=192.168.50.2:52006:X0 srcZone=LAN natSrc=14.165.177.10:58457 dstMac=08:b2:58:46:30:54 dst=15.159.150.83:443:X1 dstZone=WAN natDst=15.159.150.83:443 proto=tcp/https sent=1422 rcvd=5993 rule="6 (LAN->WAN)" app=48 dstname=example.space.dev.com arg=/ code=27 Category="Information Technology/Computers" note="Policy: a0, Info: 888 " n=3412158`,
			},
			&entry.Entry{
				Attributes: map[string]any{
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
			func(_ *Config) {},
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
			"value-contains-delimiter",
			func(_ *Config) {},
			&entry.Entry{
				Body: `test=text=abc`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"test": "text=abc",
				},
				Body: `test=text=abc`,
			},
			false,
			false,
		},
		{
			"quoted-value-contains-whitespace-delimiter",
			func(_ *Config) {},
			&entry.Entry{
				Body: `msg="Message successfully sent at 2023-12-04 06:47:31.204222276 +0000 UTC m=+5115.932279346"`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"msg": `Message successfully sent at 2023-12-04 06:47:31.204222276 +0000 UTC m=+5115.932279346`,
				},
				Body: `msg="Message successfully sent at 2023-12-04 06:47:31.204222276 +0000 UTC m=+5115.932279346"`,
			},
			false,
			false,
		},
		{
			"multiple-values-contain-delimiter",
			func(_ *Config) {},
			&entry.Entry{
				Body: `one=1=i two="2=ii" three=3=iii`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"one":   "1=i",
					"two":   "2=ii",
					"three": "3=iii",
				},
				Body: `one=1=i two="2=ii" three=3=iii`,
			},
			false,
			false,
		},
		{
			"empty-input",
			func(_ *Config) {},
			&entry.Entry{},
			&entry.Entry{},
			true,
			false,
		},
		{
			"same-delimiter-and-pair-delimiter",
			func(kv *Config) {
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
			func(kv *Config) {
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
		{
			"custom pair delimiter in quoted value",
			func(kv *Config) {
				kv.PairDelimiter = "_"
			},
			&entry.Entry{
				Body: `a=b_c="d_e"`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"a": "b",
					"c": "d_e",
				},
				Body: `a=b_c="d_e"`,
			},
			false,
			false,
		},
		{
			"embedded double quotes in single quoted value",
			func(_ *Config) {},
			&entry.Entry{
				Body: `a=b c='this is a "co ol" value'`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"a": "b",
					"c": "this is a \"co ol\" value",
				},
				Body: `a=b c='this is a "co ol" value'`,
			},
			false,
			false,
		},
		{
			"embedded double quotes end single quoted value",
			func(_ *Config) {},
			&entry.Entry{
				Body: `a=b c='this is a "co ol"'`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"a": "b",
					"c": "this is a \"co ol\"",
				},
				Body: `a=b c='this is a "co ol"'`,
			},
			false,
			false,
		},
		{
			"leading and trailing pair delimiter w/o quotes",
			func(_ *Config) {},
			&entry.Entry{
				Body: "   k1=v1   k2==v2       k3=v3= ",
			},
			&entry.Entry{
				Attributes: map[string]any{
					"k1": "v1",
					"k2": "=v2",
					"k3": "v3=",
				},
				Body: "   k1=v1   k2==v2       k3=v3= ",
			},
			false,
			false,
		},
		{
			"complicated delimiters",
			func(kv *Config) {
				kv.Delimiter = "@*"
				kv.PairDelimiter = "_!_"
			},
			&entry.Entry{
				Body: `k1@*v1_!_k2@**v2_!__k3@@*v3__`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"k1":   "v1",
					"k2":   "*v2",
					"_k3@": "v3__",
				},
				Body: `k1@*v1_!_k2@**v2_!__k3@@*v3__`,
			},
			false,
			false,
		},
		{
			"unclosed quotes",
			func(_ *Config) {},
			&entry.Entry{
				Body: `k1='v1' k2='v2`,
			},
			&entry.Entry{
				Body: `k1='v1' k2='v2`,
			},
			true,
			false,
		},
		{
			"containerd output",
			func(_ *Config) {},
			&entry.Entry{
				Body: `time="2024-11-01T12:38:17.992190505Z" level=warning msg="cleanup warnings time='2024-11-01T12:38:17Z' level=debug msg=\"starting signal loop\" namespace=moby-10000.10000 pid=1608080 runtime=io.containerd.runc.v2" namespace=moby-10000.10000`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"time":      "2024-11-01T12:38:17.992190505Z",
					"level":     "warning",
					"msg":       `cleanup warnings time='2024-11-01T12:38:17Z' level=debug msg=\"starting signal loop\" namespace=moby-10000.10000 pid=1608080 runtime=io.containerd.runc.v2`,
					"namespace": "moby-10000.10000",
				},
				Body: `time="2024-11-01T12:38:17.992190505Z" level=warning msg="cleanup warnings time='2024-11-01T12:38:17Z' level=debug msg=\"starting signal loop\" namespace=moby-10000.10000 pid=1608080 runtime=io.containerd.runc.v2" namespace=moby-10000.10000`,
			},
			false,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
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
