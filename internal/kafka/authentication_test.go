// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
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

	tokenDir := t.TempDir()
	tokenPath := filepath.Join(tokenDir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("token"), 0o600))

	saramaSASLAWSIAMOAUTHConfig := &sarama.Config{}
	saramaSASLAWSIAMOAUTHConfig.Net.SASL.Enable = true
	saramaSASLAWSIAMOAUTHConfig.Net.SASL.Mechanism = sarama.SASLTypeOAuth
	saramaSASLAWSIAMOAUTHConfig.Net.SASL.TokenProvider = &awsMSKTokenProvider{
		ctx:    t.Context(),
		region: "region",
	}

	id := component.MustNewID("oauth2client")
	oauthHost := hostWithExtensions(map[component.ID]component.Component{
		id: fakeTokenSourceExtension{},
	})

	tests := []struct {
		auth         configkafka.AuthenticationConfig
		saramaConfig *sarama.Config
		err          string
		verify       func(*testing.T, *sarama.Config)
		host         component.Host
	}{
		{
			auth: configkafka.AuthenticationConfig{
				PlainText: &configkafka.PlainTextConfig{Username: "jdoe", Password: "pass"},
			},
			saramaConfig: saramaPlaintext,
			verify: func(t *testing.T, config *sarama.Config) {
				config.Net.SASL.SCRAMClientGeneratorFunc = saramaPlaintext.Net.SASL.SCRAMClientGeneratorFunc
				assert.Equal(t, saramaPlaintext, config)
			},
		},
		{
			auth: configkafka.AuthenticationConfig{
				Kerberos: &configkafka.KerberosConfig{ServiceName: "foobar"},
			},
			saramaConfig: saramaKerberosCfg,
			verify: func(t *testing.T, config *sarama.Config) {
				assert.Equal(t, saramaKerberosCfg, config)
			},
		},
		{
			auth: configkafka.AuthenticationConfig{
				Kerberos: &configkafka.KerberosConfig{UseKeyTab: true, KeyTabPath: "/path"},
			},
			saramaConfig: saramaKerberosKeyTabCfg,
			verify: func(t *testing.T, config *sarama.Config) {
				assert.Equal(t, saramaKerberosKeyTabCfg, config)
			},
		},
		{
			auth: configkafka.AuthenticationConfig{
				Kerberos: &configkafka.KerberosConfig{ServiceName: "foobar", DisablePAFXFAST: true},
			},
			saramaConfig: saramaKerberosDisablePAFXFASTTrueCfg,
			verify: func(t *testing.T, config *sarama.Config) {
				assert.Equal(t, saramaKerberosDisablePAFXFASTTrueCfg, config)
			},
		},
		{
			auth:         configkafka.AuthenticationConfig{Kerberos: &configkafka.KerberosConfig{ServiceName: "foobar", DisablePAFXFAST: false}},
			saramaConfig: saramaKerberosDisablePAFXFASTFalseCfg,
			verify: func(t *testing.T, config *sarama.Config) {
				assert.Equal(t, saramaKerberosDisablePAFXFASTFalseCfg, config)
			},
		},
		{
			auth:         configkafka.AuthenticationConfig{SASL: &configkafka.SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-256"}},
			saramaConfig: saramaSASLSCRAM256Config,
			verify: func(t *testing.T, config *sarama.Config) {
				config.Net.SASL.SCRAMClientGeneratorFunc = saramaSASLSCRAM256Config.Net.SASL.SCRAMClientGeneratorFunc
				assert.Equal(t, saramaSASLSCRAM256Config, config)
			},
		},
		{
			auth:         configkafka.AuthenticationConfig{SASL: &configkafka.SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-512"}},
			saramaConfig: saramaSASLSCRAM512Config,
			verify: func(t *testing.T, config *sarama.Config) {
				config.Net.SASL.SCRAMClientGeneratorFunc = saramaSASLSCRAM512Config.Net.SASL.SCRAMClientGeneratorFunc
				assert.Equal(t, saramaSASLSCRAM512Config, config)
			},
		},
		{
			auth: configkafka.AuthenticationConfig{
				SASL: &configkafka.SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-512", Version: 1},
			},
			saramaConfig: saramaSASLHandshakeV1Config,
			verify: func(t *testing.T, config *sarama.Config) {
				config.Net.SASL.SCRAMClientGeneratorFunc = saramaSASLHandshakeV1Config.Net.SASL.SCRAMClientGeneratorFunc
				assert.Equal(t, saramaSASLHandshakeV1Config, config)
			},
		},
		{
			auth: configkafka.AuthenticationConfig{
				SASL: &configkafka.SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "PLAIN"},
			},
			saramaConfig: saramaSASLPLAINConfig,
			verify: func(t *testing.T, config *sarama.Config) {
				assert.Equal(t, saramaSASLPLAINConfig, config)
			},
		},
		{
			auth: configkafka.AuthenticationConfig{
				SASL: &configkafka.SASLConfig{
					Mechanism: "AWS_MSK_IAM_OAUTHBEARER", AWSMSK: configkafka.AWSMSKConfig{Region: "region"},
				},
			},
			saramaConfig: saramaSASLAWSIAMOAUTHConfig,
			verify: func(t *testing.T, config *sarama.Config) {
				assert.IsType(t, &awsMSKTokenProvider{}, config.Net.SASL.TokenProvider)
				config.Net.SASL.TokenProvider = saramaSASLAWSIAMOAUTHConfig.Net.SASL.TokenProvider
				assert.Equal(t, saramaSASLAWSIAMOAUTHConfig, config)
			},
		},
		{
			auth: configkafka.AuthenticationConfig{
				SASL: &configkafka.SASLConfig{
					Mechanism:            OAUTHBEARER,
					OAuthBearerTokenFile: tokenPath,
				},
			},
			saramaConfig: nil,
			verify: func(t *testing.T, config *sarama.Config) {
				assert.Equal(t, string(sarama.SASLTypeOAuth), string(config.Net.SASL.Mechanism))
				require.IsType(t, &saramaOAuthTokenProvider{}, config.Net.SASL.TokenProvider)
				provider := config.Net.SASL.TokenProvider.(*saramaOAuthTokenProvider)
				accessToken, err := provider.Token()
				require.NoError(t, err)
				assert.Equal(t, "token", accessToken.Token)
			},
		},
		{
			auth: configkafka.AuthenticationConfig{
				SASL: &configkafka.SASLConfig{
					Mechanism:              OAUTHBEARER,
					OAuthBearerTokenSource: id,
				},
			},
			host:         oauthHost,
			saramaConfig: nil,
			verify: func(t *testing.T, config *sarama.Config) {
				assert.Equal(t, string(sarama.SASLTypeOAuth), string(config.Net.SASL.Mechanism))
				require.IsType(t, &saramaOAuthTokenProvider{}, config.Net.SASL.TokenProvider)
				provider := config.Net.SASL.TokenProvider.(*saramaOAuthTokenProvider)
				accessToken, err := provider.Token()
				require.NoError(t, err)
				assert.Equal(t, "ext-token", accessToken.Token)
			},
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			config := &sarama.Config{}
			host := test.host
			if host == nil {
				host = hostWithExtensions(nil)
			}
			err := configureSaramaAuthentication(t.Context(), test.auth, config, host)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
				return
			}
			require.NoError(t, err)
			if test.verify != nil {
				test.verify(t, config)
			} else {
				config.Net.SASL.SCRAMClientGeneratorFunc = test.saramaConfig.Net.SASL.SCRAMClientGeneratorFunc
				assert.Equal(t, test.saramaConfig, config)
			}
		})
	}
}

func TestSaramaOAuthMissingExtension(t *testing.T) {
	t.Parallel()
	cfg := configkafka.AuthenticationConfig{
		SASL: &configkafka.SASLConfig{
			Mechanism:              OAUTHBEARER,
			OAuthBearerTokenSource: component.MustNewID("missing"),
		},
	}
	saramaCfg := &sarama.Config{}
	host := hostWithExtensions(nil)
	err := configureSaramaAuthentication(t.Context(), cfg, saramaCfg, host)
	require.ErrorContains(t, err, "oauth token source extension \"missing\" not found")
}

func TestSaramaOAuthTokenProvider_RejectsEmptyToken(t *testing.T) {
	t.Parallel()

	p := &saramaOAuthTokenProvider{
		ctx:      t.Context(),
		provider: stubTokenProvider{token: ""},
	}
	_, err := p.Token()
	require.ErrorIs(t, err, ErrEmptyToken)
}
