// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"cloud.google.com/go/compute/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
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
		config:           config,
		newIDTokenSource: idtoken.NewTokenSource,
		getDefaultEmail:  defaultServiceAccountEmail,
	}
	return ca, nil
}

// clientAuthenticator supplies credentials from an oauth.TokenSource.
type clientAuthenticator struct {
	*oauth.TokenSource
	config *Config

	// principal is the subject of google_managed_kafka tokens, resolved on Start.
	principal string

	newIDTokenSource func(ctx context.Context, audience string, opts ...idtoken.ClientOption) (oauth2.TokenSource, error)
	getDefaultEmail  func(ctx context.Context) (string, error)
}

func (ca *clientAuthenticator) Start(ctx context.Context, _ component.Host) error {
	config := ca.config
	creds, err := google.FindDefaultCredentials(ctx, config.Scopes...)
	if err != nil {
		return err
	}
	// Allow users to override the Project ID using environment variables
	if config.Project == "" {
		config.Project = os.Getenv(envProjectID)
	}
	if config.Project == "" {
		config.Project = creds.ProjectID
	}
	if config.Project == "" {
		return errors.New("no project set in config or found with application default credentials")
	}
	if config.QuotaProject == "" {
		config.QuotaProject = quotaProjectFromCreds(creds)
	}
	if config.TokenType == googleManagedKafka {
		ca.principal, err = ca.resolvePrincipal(ctx, creds)
		if err != nil {
			return err
		}
	}
	source, err := ca.newTokenSource(ctx, creds)
	if err != nil {
		return err
	}
	ca.TokenSource = &oauth.TokenSource{TokenSource: source}
	return nil
}

// resolvePrincipal returns the principal from the config, else the client_email of a service
// account key, else the default service account email from the metadata server.
func (ca *clientAuthenticator) resolvePrincipal(ctx context.Context, creds *google.Credentials) (string, error) {
	if ca.config.Principal != "" {
		return ca.config.Principal, nil
	}
	if len(creds.JSON) == 0 {
		email, err := ca.getDefaultEmail(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get the default service account email from the metadata server, set principal in the config: %w", err)
		}
		return email, nil
	}
	var v struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(creds.JSON, &v); err == nil && v.ClientEmail != "" {
		return v.ClientEmail, nil
	}
	return "", errors.New("no principal found in application default credentials, set principal in the config")
}

func defaultServiceAccountEmail(ctx context.Context) (string, error) {
	return metadata.EmailWithContext(ctx, "default")
}

func (ca *clientAuthenticator) newTokenSource(ctx context.Context, creds *google.Credentials) (oauth2.TokenSource, error) {
	switch ca.config.TokenType {
	case idToken:
		opts := []idtoken.ClientOption{}

		if len(creds.JSON) > 0 {
			opts = append(opts, option.WithCredentials(creds))
		}

		return ca.newIDTokenSource(ctx, ca.config.Audience, opts...)
	default:
		return creds.TokenSource, nil
	}
}

func (*clientAuthenticator) Shutdown(_ context.Context) error {
	return nil
}

// quotaProjectFromCreds retrieves quota project from the credentials file.
// Based on how the google go client gets quota project:
// https://github.com/googleapis/google-api-go-client/blob/113082d14d54f188d1b6c34c652e416592fc51b5/internal/creds.go#L159
func quotaProjectFromCreds(creds *google.Credentials) string {
	if creds == nil {
		return ""
	}
	var v struct {
		QuotaProject string `json:"quota_project_id"`
	}
	if err := json.Unmarshal(creds.JSON, &v); err != nil {
		return ""
	}
	return v.QuotaProject
}
