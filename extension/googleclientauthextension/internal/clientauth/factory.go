// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		config:                        config,
		newIDTokenCredentials:         idtoken.NewCredentials,
		newIDTokenCredentialsFromJSON: idtoken.NewCredentialsFromJSON,
	}
	return ca, nil
}

// clientAuthenticator supplies credentials from an oauth.TokenSource.
type clientAuthenticator struct {
	*oauth.TokenSource
	config *Config

	newIDTokenCredentials         func(opts *idtoken.Options) (*auth.Credentials, error)
	newIDTokenCredentialsFromJSON func(credType credentials.CredType, b []byte, opts *idtoken.Options) (*auth.Credentials, error)
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
		if config.Project, err = creds.ProjectID(ctx); err != nil {
			return fmt.Errorf("failed to get project from application default credentials: %w", err)
		}
	}
	if config.Project == "" {
		return errors.New("no project set in config or found with application default credentials")
	}
	if config.QuotaProject == "" {
		if config.QuotaProject, err = creds.QuotaProjectID(ctx); err != nil {
			return fmt.Errorf("failed to get quota project from application default credentials: %w", err)
		}
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
		opts := &idtoken.Options{Audience: ca.config.Audience}
		var err error
		// Reuse the credentials file selected by ADC so ID tokens come from
		// the same identity as access tokens. Without it, idtoken would skip
		// the gcloud well-known file and use the metadata server on GCE.
		if b := creds.JSON(); len(b) > 0 {
			var f struct {
				Type string `json:"type"`
			}
			if err = json.Unmarshal(b, &f); err != nil {
				return nil, fmt.Errorf("failed to parse credentials type: %w", err)
			}
			creds, err = ca.newIDTokenCredentialsFromJSON(credentials.CredType(f.Type), b, opts)
		} else {
			creds, err = ca.newIDTokenCredentials(opts)
		}
		if err != nil {
			return nil, err
		}
	}
	return oauth2adapt.TokenSourceFromTokenProvider(creds), nil
}

func (*clientAuthenticator) Shutdown(_ context.Context) error {
	return nil
}
