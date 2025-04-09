// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

func TestAuthentication(t *testing.T) {
	saramaPlaintext := &sarama.Config{}
	saramaPlaintext.Net.SASL.Enable = true
	saramaPlaintext.Net.SASL.User = "jdoe"
	saramaPlaintext.Net.SASL.Password = "pass"

	saramaSASLSCRAM256Config := &sarama.Config{}
	saramaSASLSCRAM256Config.Net.SASL.Enable = true
	saramaSASLSCRAM256Config.Net.SASL.User = "jdoe"
	saramaSASLSCRAM256Config.Net.SASL.Password = "pass"
	saramaSASLSCRAM256Config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256

	saramaSASLSCRAM512Config := &sarama.Config{}
	saramaSASLSCRAM512Config.Net.SASL.Enable = true
	saramaSASLSCRAM512Config.Net.SASL.User = "jdoe"
	saramaSASLSCRAM512Config.Net.SASL.Password = "pass"
	saramaSASLSCRAM512Config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512

	saramaSASLHandshakeV1Config := &sarama.Config{}
	saramaSASLHandshakeV1Config.Net.SASL.Enable = true
	saramaSASLHandshakeV1Config.Net.SASL.User = "jdoe"
	saramaSASLHandshakeV1Config.Net.SASL.Password = "pass"
	saramaSASLHandshakeV1Config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	saramaSASLHandshakeV1Config.Net.SASL.Version = sarama.SASLHandshakeV1

	saramaSASLPLAINConfig := &sarama.Config{}
	saramaSASLPLAINConfig.Net.SASL.Enable = true
	saramaSASLPLAINConfig.Net.SASL.User = "jdoe"
	saramaSASLPLAINConfig.Net.SASL.Password = "pass"
	saramaSASLPLAINConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext

	saramaSASLAWSIAMOAUTHConfig := &sarama.Config{}
	saramaSASLAWSIAMOAUTHConfig.Net.SASL.Enable = true
	saramaSASLAWSIAMOAUTHConfig.Net.SASL.Mechanism = sarama.SASLTypeOAuth
	saramaSASLAWSIAMOAUTHConfig.Net.SASL.TokenProvider = &awsMSKTokenProvider{
		ctx:    context.Background(),
		region: "region",
	}

	saramaKerberosCfg := &sarama.Config{}
	saramaKerberosCfg.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	saramaKerberosCfg.Net.SASL.Enable = true
	saramaKerberosCfg.Net.SASL.GSSAPI.ServiceName = "foobar"
	saramaKerberosCfg.Net.SASL.GSSAPI.AuthType = sarama.KRB5_USER_AUTH

	saramaKerberosKeyTabCfg := &sarama.Config{}
	saramaKerberosKeyTabCfg.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	saramaKerberosKeyTabCfg.Net.SASL.Enable = true
	saramaKerberosKeyTabCfg.Net.SASL.GSSAPI.KeyTabPath = "/path"
	saramaKerberosKeyTabCfg.Net.SASL.GSSAPI.AuthType = sarama.KRB5_KEYTAB_AUTH

	saramaKerberosDisablePAFXFASTTrueCfg := &sarama.Config{}
	saramaKerberosDisablePAFXFASTTrueCfg.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	saramaKerberosDisablePAFXFASTTrueCfg.Net.SASL.Enable = true
	saramaKerberosDisablePAFXFASTTrueCfg.Net.SASL.GSSAPI.ServiceName = "foobar"
	saramaKerberosDisablePAFXFASTTrueCfg.Net.SASL.GSSAPI.AuthType = sarama.KRB5_USER_AUTH
	saramaKerberosDisablePAFXFASTTrueCfg.Net.SASL.GSSAPI.DisablePAFXFAST = true

	saramaKerberosDisablePAFXFASTFalseCfg := &sarama.Config{}
	saramaKerberosDisablePAFXFASTFalseCfg.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	saramaKerberosDisablePAFXFASTFalseCfg.Net.SASL.Enable = true
	saramaKerberosDisablePAFXFASTFalseCfg.Net.SASL.GSSAPI.ServiceName = "foobar"
	saramaKerberosDisablePAFXFASTFalseCfg.Net.SASL.GSSAPI.AuthType = sarama.KRB5_USER_AUTH
	saramaKerberosDisablePAFXFASTFalseCfg.Net.SASL.GSSAPI.DisablePAFXFAST = false

	tests := []struct {
		auth         configkafka.AuthenticationConfig
		saramaConfig *sarama.Config
		err          string
	}{
		{
			auth: configkafka.AuthenticationConfig{
				PlainText: &configkafka.PlainTextConfig{Username: "jdoe", Password: "pass"},
			},
			saramaConfig: saramaPlaintext,
		},
		{
			auth: configkafka.AuthenticationConfig{
				Kerberos: &configkafka.KerberosConfig{ServiceName: "foobar"},
			},
			saramaConfig: saramaKerberosCfg,
		},
		{
			auth: configkafka.AuthenticationConfig{
				Kerberos: &configkafka.KerberosConfig{UseKeyTab: true, KeyTabPath: "/path"},
			},
			saramaConfig: saramaKerberosKeyTabCfg,
		},
		{
			auth: configkafka.AuthenticationConfig{
				Kerberos: &configkafka.KerberosConfig{ServiceName: "foobar", DisablePAFXFAST: true},
			},
			saramaConfig: saramaKerberosDisablePAFXFASTTrueCfg,
		},
		{
			auth:         configkafka.AuthenticationConfig{Kerberos: &configkafka.KerberosConfig{ServiceName: "foobar", DisablePAFXFAST: false}},
			saramaConfig: saramaKerberosDisablePAFXFASTFalseCfg,
		},
		{
			auth:         configkafka.AuthenticationConfig{SASL: &configkafka.SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-256"}},
			saramaConfig: saramaSASLSCRAM256Config,
		},
		{
			auth:         configkafka.AuthenticationConfig{SASL: &configkafka.SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-512"}},
			saramaConfig: saramaSASLSCRAM512Config,
		},
		{
			auth: configkafka.AuthenticationConfig{
				SASL: &configkafka.SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-512", Version: 1},
			},
			saramaConfig: saramaSASLHandshakeV1Config,
		},
		{
			auth: configkafka.AuthenticationConfig{
				SASL: &configkafka.SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "PLAIN"},
			},
			saramaConfig: saramaSASLPLAINConfig,
		},
		{
			auth: configkafka.AuthenticationConfig{
				SASL: &configkafka.SASLConfig{
					Mechanism: "AWS_MSK_IAM_OAUTHBEARER", AWSMSK: configkafka.AWSMSKConfig{Region: "region"},
				},
			},
			saramaConfig: saramaSASLAWSIAMOAUTHConfig,
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			config := &sarama.Config{}
			configureSaramaAuthentication(context.Background(), test.auth, config)

			// equalizes SCRAMClientGeneratorFunc to do assertion with the same reference.
			config.Net.SASL.SCRAMClientGeneratorFunc = test.saramaConfig.Net.SASL.SCRAMClientGeneratorFunc
			assert.Equal(t, test.saramaConfig, config)
		})
	}
}
