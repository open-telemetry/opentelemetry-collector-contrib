// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/aws/aws-msk-iam-sasl-signer-go/signer"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// configureSaramaAuthentication configures authentication in sarama.Config.
//
// The provided config is assumed to have been validated.

func configureSaramaAuthentication(
	ctx context.Context,
	config configkafka.AuthenticationConfig,
	saramaConfig *sarama.Config,
	host component.Host,
) error {
	if config.PlainText != nil {
		configurePlaintext(*config.PlainText, saramaConfig)
	}
	if config.SASL != nil {
		if err := configureSASL(ctx, *config.SASL, saramaConfig, host); err != nil {
			return err
		}
	}
	if config.Kerberos != nil {
		configureKerberos(*config.Kerberos, saramaConfig)
	}
	return nil
}

func configurePlaintext(config configkafka.PlainTextConfig, saramaConfig *sarama.Config) {
	saramaConfig.Net.SASL.Enable = true
	saramaConfig.Net.SASL.User = config.Username
	saramaConfig.Net.SASL.Password = config.Password
}

func configureSASL(ctx context.Context, config configkafka.SASLConfig, saramaConfig *sarama.Config, host component.Host) error {
	saramaConfig.Net.SASL.Enable = true
	saramaConfig.Net.SASL.Version = int16(config.Version)

	switch config.Mechanism {
	case SCRAMSHA512:
		saramaConfig.Net.SASL.User = config.Username
		saramaConfig.Net.SASL.Password = config.Password
		saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: sha512.New} }
		saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	case SCRAMSHA256:
		saramaConfig.Net.SASL.User = config.Username
		saramaConfig.Net.SASL.Password = config.Password
		saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: sha256.New} }
		saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
	case PLAIN:
		saramaConfig.Net.SASL.User = config.Username
		saramaConfig.Net.SASL.Password = config.Password
		saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	case AWSMSKIAMOAUTHBEARER:
		saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeOAuth
		saramaConfig.Net.SASL.TokenProvider = &awsMSKTokenProvider{ctx: ctx, region: config.AWSMSK.Region}
	case OAUTHBEARER:
		tp, err := resolveOAuthBearerProvider(context.WithoutCancel(ctx), &config, host)
		if err != nil {
			return err
		}
		saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeOAuth
		saramaConfig.Net.SASL.TokenProvider = &saramaOAuthTokenProvider{ctx: context.WithoutCancel(ctx), provider: tp}
	default:
		return fmt.Errorf("unsupported SASL mechanism: %s", config.Mechanism)
	}
	return nil
}

func configureKerberos(config configkafka.KerberosConfig, saramaConfig *sarama.Config) {
	saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	saramaConfig.Net.SASL.Enable = true
	if config.UseKeyTab {
		saramaConfig.Net.SASL.GSSAPI.KeyTabPath = config.KeyTabPath
		saramaConfig.Net.SASL.GSSAPI.AuthType = sarama.KRB5_KEYTAB_AUTH
	} else {
		saramaConfig.Net.SASL.GSSAPI.AuthType = sarama.KRB5_USER_AUTH
		saramaConfig.Net.SASL.GSSAPI.Password = config.Password
	}
	saramaConfig.Net.SASL.GSSAPI.KerberosConfigPath = config.ConfigPath
	saramaConfig.Net.SASL.GSSAPI.Username = config.Username
	saramaConfig.Net.SASL.GSSAPI.Realm = config.Realm
	saramaConfig.Net.SASL.GSSAPI.ServiceName = config.ServiceName
	saramaConfig.Net.SASL.GSSAPI.DisablePAFXFAST = config.DisablePAFXFAST
}

type awsMSKTokenProvider struct {
	ctx    context.Context
	region string
}

// Token return the AWS session token for the AWS_MSK_IAM_OAUTHBEARER mechanism
func (c *awsMSKTokenProvider) Token() (*sarama.AccessToken, error) {
	token, _, err := signer.GenerateAuthToken(c.ctx, c.region)
	return &sarama.AccessToken{Token: token}, err
}

type saramaOAuthTokenProvider struct {
	ctx      context.Context
	provider TokenProvider
}

func (s *saramaOAuthTokenProvider) Token() (*sarama.AccessToken, error) {
	// Sarama does not pass a context into the token provider callback. To keep parity with the
	// franz-go path, we preserve and use the context from the configuration / start call.
	token, err := s.provider.Token(s.ctx)
	if err != nil {
		return nil, err
	}
	// Defensive check: our built-in TokenProviders already fail on empty tokens, but custom
	// TokenProviders may not.
	if token == "" {
		return nil, ErrEmptyToken
	}
	return &sarama.AccessToken{Token: token}, nil
}
