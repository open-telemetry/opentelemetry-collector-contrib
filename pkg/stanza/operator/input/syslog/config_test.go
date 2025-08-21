// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog

import (
	"path/filepath"
	"testing"

	"go.opentelemetry.io/collector/config/configoptional"
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
				Name:               "default",
				ExpectUnmarshalErr: false,
				Expect:             NewConfig(),
			},
			{
				Name:               "tcp",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = "rfc3164"
					cfg.Location = "foo"
					cfg.EnableOctetCounting = true
					tcpConfig := tcp.NewConfig().BaseConfig
					tcpConfig.MaxLogSize = 1000000
					tcpConfig.ListenAddress = "10.0.0.1:9000"
					tcpConfig.AddAttributes = true
					tcpConfig.Encoding = "utf-16"
					tcpConfig.SplitConfig.LineStartPattern = "ABC"
					tcpConfig.SplitConfig.LineEndPattern = ""
					tcpConfig.TLS = configoptional.Some(configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "foo",
							KeyFile:  "foo2",
							CAFile:   "foo3",
						},
						ClientCAFile: "foo4",
					})
					cfg.TCP = configoptional.Some(tcpConfig)
					return cfg
				}(),
			},
			{
				Name:               "udp",
				ExpectUnmarshalErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = "rfc5424"
					cfg.Location = "foo"
					cfg.OnError = "send_quiet"
					udpConfig := udp.NewConfig().BaseConfig
					udpConfig.ListenAddress = "10.0.0.1:9000"
					udpConfig.AddAttributes = true
					udpConfig.Encoding = "utf-16"
					udpConfig.SplitConfig.LineStartPattern = "ABC"
					udpConfig.SplitConfig.LineEndPattern = ""
					cfg.UDP = configoptional.Some(udpConfig)
					return cfg
				}(),
			},
		},
	}.Run(t)
}
