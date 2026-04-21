// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cfgardenobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver"

import (
	"errors"
	"fmt"
	"time"
)

// Config defines configuration for CF Garden observer.
type Config struct {
	// CloudFoundry API Configuration
	CloudFoundry CfConfig `mapstructure:"cloud_foundry"`

	// Garden API Configuration
	Garden GardenConfig `mapstructure:"garden"`

	// RefreshInterval determines the frequency at which the observer
	// needs to poll for collecting information about new processes.
	// Default: "1m"
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`

	// The time to wait before resyncing app information on cached containers
	// using the CloudFoundry API.
	// Default: "5m"
	CacheSyncInterval time.Duration `mapstructure:"cache_sync_interval"`

	// Determines whether or not Application labels get added to the Endpoint labels.
	// This requires cloud_foundry to be configured, such that API calls can be made
	// Default: false
	IncludeAppLabels bool `mapstructure:"include_app_labels"`
}

// Validate overrides the embedded noop validation so that load config can trigger
// our own validation logic.
func (config *Config) Validate() error {
	if !config.IncludeAppLabels {
		return nil
	}

	c := config.CloudFoundry
	if c.Endpoint == "" {
		return errors.New("CloudFoundry.Endpoint must be specified when IncludeAppLabels is set to true")
	}
	if c.Auth.Type == "" {
		return errors.New("CloudFoundry.Auth.Type must be specified when IncludeAppLabels is set to true")
	}

	switch c.Auth.Type {
	case authTypeUserPass:
		if c.Auth.Username == "" {
			return fieldError(authTypeUserPass, "username")
		}
		if c.Auth.Password == "" {
			return fieldError(authTypeUserPass, "password")
		}
	case authTypeClientCredentials:
		if c.Auth.ClientID == "" {
			return fieldError(authTypeClientCredentials, "client_id")
		}
		if c.Auth.ClientSecret == "" {
			return fieldError(authTypeClientCredentials, "client_secret")
		}
	case authTypeToken:
		if c.Auth.AccessToken == "" {
			return fieldError(authTypeToken, "access_token")
		}
		if c.Auth.RefreshToken == "" {
			return fieldError(authTypeToken, "refresh_token")
		}
	default:
		return fmt.Errorf("configuration option `auth_type` must be set to one of the following values: [user_pass, client_credentials, token]. Specified value: %s", c.Auth.Type)
	}

	return nil
}

func fieldError(authType authType, param string) error {
	return fmt.Errorf("%s is required when using auth_type: %s", param, authType)
}

type GardenConfig struct {
	// The URL of the CF Garden api. Default is "/var/vcap/data/garden/garden.sock"
	Endpoint string `mapstructure:"endpoint"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type CfConfig struct {
	// The URL of the CloudFoundry API
	Endpoint string `mapstructure:"endpoint"`

	// Authentication details
	Auth CfAuth `mapstructure:"auth"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type CfAuth struct {
	// Authentication method, there are 3 options
	Type authType `mapstructure:"type"`

	// Used for user_pass authentication method
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`

	// Used for token authentication method
	AccessToken  string `mapstructure:"access_token"`
	RefreshToken string `mapstructure:"refresh_token"`

	// Used for client_credentials authentication method
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

// authType describes the type of authentication to use for the CloudFoundry API
type authType string

const (
	// authTypeClientCredentials uses a client ID and client secret to authenticate
	authTypeClientCredentials authType = "client_credentials"
	// authTypeUserPass uses username and password to authenticate
	authTypeUserPass authType = "user_pass"
	// authTypeToken uses access token and refresh token to authenticate
	authTypeToken authType = "token"
)
