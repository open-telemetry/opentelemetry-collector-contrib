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
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`

	// The time to wait before resyncing app information on cached containers
	// using the CloudFoundry API. Example: cache_sync_interval: "20m"
	// Default: "5m"
	CacheSyncInterval time.Duration `mapstructure:"cache_sync_interval"`
}

// Validate overrides the embedded noop validation so that load config can trigger
// our own validation logic.
func (config *Config) Validate() error {
	c := config.CloudFoundry
	if c.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	if c.AuthType == "" {
		return errors.New("config.AuthType must be specified")
	}

	switch c.AuthType {
	case AuthTypeUserPass:
		if c.Username == "" {
			return fieldError(AuthTypeUserPass, "username")
		}
		if c.Password == "" {
			return fieldError(AuthTypeUserPass, "password")
		}
	case AuthTypeClientCredentials:
		if c.ClientID == "" {
			return fieldError(AuthTypeClientCredentials, "client_id")
		}
		if c.ClientSecret == "" {
			return fieldError(AuthTypeClientCredentials, "client_secret")
		}
	case AuthTypeToken:
		if c.AccessToken == "" {
			return fieldError(AuthTypeToken, "access_token")
		}
		if c.RefreshToken == "" {
			return fieldError(AuthTypeToken, "refresh_token")
		}
	default:
		return fmt.Errorf("unknown auth_type: %s", c.AuthType)
	}

	return nil
}

func fieldError(authType AuthType, param string) error {
	return fmt.Errorf("%s is required when using auth_type: %s", param, authType)
}

type GardenConfig struct {
	// The URL of the CF Garden api. Default is "/var/vcap/data/garden/garden.sock"
	Endpoint string `mapstructure:"endpoint"`
}

type CfConfig struct {
	// The URL of the CloudFoundry API
	Endpoint string `mapstructure:"endpoint"`

	// Authentication method, there are 3 options
	AuthType AuthType `mapstructure:"auth_type"`

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

// AuthType describes the type of authentication to use for the CloudFoundry API
type AuthType string

const (
	// AuthTypeClientCredentials uses a client ID and client secret to authenticate
	AuthTypeClientCredentials AuthType = "client_credentials"
	// AuthTypeUserPass uses username and password to authenticate
	AuthTypeUserPass AuthType = "user_pass"
	// AuthTypeToken uses access token and refresh token to authenticate
	AuthTypeToken AuthType = "token"
)
