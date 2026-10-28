// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"context"
	"errors"
	"os"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/credentials/idtoken"
	"cloud.google.com/go/auth/oauth2adapt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/credentials/oauth"
)

const (
	// Choose a well-known environment variable for allowing project ID
	// override.
	// This environment variable is also used by Google Auth library for Go
	// (https://pkg.go.dev/cloud.google.com/go/auth) in project ID detection
	// process.
	envProjectID = "GOOGLE_CLOUD_PROJECT"
)

func CreateExtension(_ context.Context, _ extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	ca := &clientAuthenticator{
		config:                config,
		newIDTokenCredentials: idtoken.NewCredentials,
	}
	return ca, nil
}

// clientAuthenticator supplies credentials from an oauth.TokenSource.
type clientAuthenticator struct {
	*oauth.TokenSource
	config *Config

	newIDTokenCredentials func(opts *idtoken.Options) (*auth.Credentials, error)
}

func (ca *clientAuthenticator) Start(ctx context.Context, _ component.Host) error {
	config := ca.config
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes: config.Scopes,
	})
	if err != nil {
		return err
	}
	// Allow users to override the Project ID using environment variables
	if config.Project == "" {
		config.Project = os.Getenv(envProjectID)
	}
	if config.Project == "" {
		config.Project, _ = creds.ProjectID(ctx)
	}
	if config.Project == "" {
		return errors.New("no project set in config or found with application default credentials")
	}
	if config.QuotaProject == "" {
		config.QuotaProject, _ = creds.QuotaProjectID(ctx)
	}
	source, err := ca.newTokenSource(creds)
	if err != nil {
		return err
	}
	ca.TokenSource = &oauth.TokenSource{TokenSource: source}
	return nil
}

func (ca *clientAuthenticator) newTokenSource(creds *auth.Credentials) (oauth2.TokenSource, error) {
	if ca.config.TokenType == idToken {
		var err error
		creds, err = ca.newIDTokenCredentials(&idtoken.Options{
			Audience: ca.config.Audience,
		})
		if err != nil {
			return nil, err
		}
	}
	return oauth2adapt.TokenSourceFromTokenProvider(creds), nil
}

func (*clientAuthenticator) Shutdown(_ context.Context) error {
	return nil
}
