// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"golang.org/x/oauth2"
)

// googleManagedKafkaHeader is the header of the token expected by Google Cloud Managed Service
// for Apache Kafka. It is not signed: the access token it carries is the credential.
const googleManagedKafkaHeader = `{"typ":"JWT","alg":"GOOG_OAUTH2_TOKEN"}`

// contextTokenSource is the interface used by the Kafka components to get
// SASL/OAUTHBEARER tokens from an extension.
type contextTokenSource interface {
	Token(context.Context) (*oauth2.Token, error)
}

var _ contextTokenSource = (*clientAuthenticator)(nil)

type googleManagedKafkaClaims struct {
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
}

// Token returns the token for the configured token_type. For google_managed_kafka, the
// access token is wrapped in the format expected by Google Cloud Managed Service for Apache Kafka.
func (ca *clientAuthenticator) Token(ctx context.Context) (*oauth2.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ca.TokenSource == nil {
		return nil, errors.New("not started")
	}
	token, err := ca.TokenSource.Token()
	if err != nil {
		return nil, err
	}
	if ca.config.TokenType != googleManagedKafka {
		return token, nil
	}
	return wrapGoogleManagedKafkaToken(token, ca.principal, time.Now())
}

func wrapGoogleManagedKafkaToken(token *oauth2.Token, principal string, now time.Time) (*oauth2.Token, error) {
	claims, err := json.Marshal(googleManagedKafkaClaims{
		ExpiresAt: token.Expiry.Unix(),
		IssuedAt:  now.Unix(),
		Issuer:    "Google",
		Subject:   principal,
	})
	if err != nil {
		return nil, err
	}
	enc := base64.RawURLEncoding.EncodeToString
	return &oauth2.Token{
		AccessToken: enc([]byte(googleManagedKafkaHeader)) + "." + enc(claims) + "." + enc([]byte(token.AccessToken)),
		TokenType:   "Bearer",
		Expiry:      token.Expiry,
	}, nil
}
