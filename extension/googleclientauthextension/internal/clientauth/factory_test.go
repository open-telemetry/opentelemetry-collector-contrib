// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

type mockIDTokenSource struct {
	token string
}

func (ts *mockIDTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: ts.token,
	}, nil
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExtension(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
}

func TestStart(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.NoError(t, err)
}

func TestStart_WithError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/foo.json")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.Error(t, err)
}

func TestStart_WithNoProjectError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds_no_project.json")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.Error(t, err)
}

func TestStart_WithProjectFromEnvVar(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds_no_project.json")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.NoError(t, err)
}

func TestStart_WithProjectOverride(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-overridden-project")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.NoError(t, err)
	// verify the value of the overridden project id
	ca, ok := ext.(*clientAuthenticator)
	if !ok {
		t.Fatalf("Returned extension is not of type *clientAuthenticator. Got: %T", ext)
	}
	assert.Equal(t, "my-overridden-project", ca.config.Project)
}

func TestStart_idtoken(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ca := &clientAuthenticator{
		config: &Config{
			Project: "my-project",
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/logging.write",
				"https://www.googleapis.com/auth/monitoring.write",
				"https://www.googleapis.com/auth/trace.append",
			},
			TokenType: idToken,
			Audience:  "my-audience",
		},
		newIDTokenSource: func(_ context.Context, _ string, opts ...idtoken.ClientOption) (oauth2.TokenSource, error) {
			// opts should have option.WithCredentials
			assert.Len(t, opts, 1)

			return &mockIDTokenSource{token: "dummy token"}, nil
		},
	}
	err := ca.Start(t.Context(), nil)
	assert.NoError(t, err)

	token, err := ca.Token()
	assert.NoError(t, err)
	assert.Equal(t, "dummy token", token.AccessToken)
}

func Test_newTokenSource_idtokenWithoutCredsJSON(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	ca := &clientAuthenticator{
		config: &Config{
			Project: "my-project",
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/logging.write",
				"https://www.googleapis.com/auth/monitoring.write",
				"https://www.googleapis.com/auth/trace.append",
			},
			TokenType: idToken,
			Audience:  "my-audience",
		},
		newIDTokenSource: func(_ context.Context, _ string, opts ...idtoken.ClientOption) (oauth2.TokenSource, error) {
			// opts should have no item
			assert.Empty(t, opts)

			return &mockIDTokenSource{token: "dummy token"}, nil
		},
	}
	ts, err := ca.newTokenSource(t.Context(), &google.Credentials{
		ProjectID: "my-project",
	})
	assert.NotNil(t, ts)
	assert.NoError(t, err)

	token, err := ts.Token()
	assert.NoError(t, err)
	assert.Equal(t, "dummy token", token.AccessToken)
}
