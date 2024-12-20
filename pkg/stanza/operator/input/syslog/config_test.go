// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:      "default",
				ExpectErr: false,
				Expect:    NewConfig(),
			},
			{
				Name:      "tcp",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = "rfc3164"
					cfg.Location = "foo"
					cfg.EnableOctetCounting = true
					cfg.TCP = &tcp.NewConfig().BaseConfig
					cfg.TCP.MaxLogSize = 1000000
					cfg.TCP.ListenAddress = "10.0.0.1:9000"
					cfg.TCP.AddAttributes = true
					cfg.TCP.Encoding = "utf-16"
					cfg.TCP.SplitConfig.LineStartPattern = "ABC"
					cfg.TCP.SplitConfig.LineEndPattern = ""
					cfg.TCP.TLS = &configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "foo",
							KeyFile:  "foo2",
							CAFile:   "foo3",
						},
						ClientCAFile: "foo4",
					}
					return cfg
				}(),
			},
			{
				Name:      "udp",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = "rfc5424"
					cfg.Location = "foo"
					cfg.UDP = &udp.NewConfig().BaseConfig
					cfg.UDP.ListenAddress = "10.0.0.1:9000"
					cfg.UDP.AddAttributes = true
					cfg.UDP.Encoding = "utf-16"
					cfg.UDP.SplitConfig.LineStartPattern = "ABC"
					cfg.UDP.SplitConfig.LineEndPattern = ""
					return cfg
				}(),
			},
			{
				Name:      "with_parser_config",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = "rfc5424"
					cfg.Location = "foo"
					cfg.ParserConfig.OnError = "drop"
					cfg.ParserConfig.ParseFrom = entry.NewBodyField("from")
					cfg.ParserConfig.ParseTo = entry.RootableField{Field: entry.NewBodyField("log")}
					parseField := entry.NewBodyField("severity_field")
					severityParser := helper.NewSeverityConfig()
					severityParser.ParseFrom = &parseField
					mapping := map[string]any{
						"critical": "5xx",
						"error":    "4xx",
						"info":     "3xx",
						"debug":    "2xx",
					}
					severityParser.Mapping = mapping
					cfg.SeverityConfig = &severityParser
					return cfg
				}(),
			},
		},
	}.Run(t)
}
