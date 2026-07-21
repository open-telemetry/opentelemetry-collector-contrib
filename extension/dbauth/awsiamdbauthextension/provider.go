// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package awsiamdbauthextension provides AWS RDS IAM database authentication for
// the db_auth framework. It is a Collector extension that also implements
// dbauth.Provider: it mints short-lived RDS IAM auth tokens and supplies them as
// the connection secret. Receivers reference it by component ID through their
// db_auth block and resolve it from the host extension map.
package awsiamdbauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth/awsiamdbauthextension"

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

// rdsTokenLifetime is the lifetime AWS gives an RDS IAM auth token.
const rdsTokenLifetime = 15 * time.Minute

// iamExtension is the aws_iam provider. It is a Collector extension (so it lives
// in the host extension map) that also implements dbauth.Provider. It mints
// short-lived RDS IAM auth tokens on demand and supplies them as the connection
// secret.
type iamExtension struct {
	cfg       *Config
	awsConfig aws.Config
}

var (
	_ extension.Extension = (*iamExtension)(nil)
	_ dbauth.Provider     = (*iamExtension)(nil)
)

func (*iamExtension) Start(context.Context, component.Host) error { return nil }
func (*iamExtension) Shutdown(context.Context) error              { return nil }

// GetCredential mints an RDS IAM auth token for the resolved endpoint and user
// and returns it as the Secret. Username is nil — the consumer uses its own
// configured username.
//
// The endpoint and database user are taken from the per-connection dbauth.Request
// when it supplies them; otherwise they fall back to the extension's own config.
// The region has no request source and comes only from the extension config,
// where it is required (validated at load).
func (e *iamExtension) GetCredential(ctx context.Context, req dbauth.Request) (*dbauth.Credential, error) {
	endpoint := e.cfg.Endpoint
	if req.Endpoint != "" {
		endpoint = req.Endpoint
	}
	dbUser := e.cfg.DBUser
	if req.Username != "" {
		dbUser = req.Username
	}

	issuedAt := time.Now()
	token, err := auth.BuildAuthToken(ctx, endpoint, e.awsConfig.Region, dbUser, e.awsConfig.Credentials)
	if err != nil {
		return nil, fmt.Errorf("aws_iam: mint RDS token for %q: %w", endpoint, err)
	}
	notAfter := issuedAt.Add(rdsTokenLifetime)
	return &dbauth.Credential{
		Secret:   token,
		NotAfter: &notAfter,
	}, nil
}
