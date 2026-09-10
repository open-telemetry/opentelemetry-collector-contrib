// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

// AuthType selects how the receiver authenticates to the Oracle database.
type AuthType string

const (
	// AuthTypePassword uses the username/password fields (the default).
	AuthTypePassword AuthType = "password"
	// AuthTypeKerberos uses the kerberos block for external Kerberos authentication.
	AuthTypeKerberos AuthType = "kerberos"
)

// KerberosCredentialType selects how the Kerberos client obtains its credentials.
type KerberosCredentialType string

const (
	// KerberosCredentialKeytab reads the principal's long-term key from a keytab file.
	KerberosCredentialKeytab KerberosCredentialType = "keytab"
	// KerberosCredentialCache reuses an existing ticket (TGT) from a credential cache.
	KerberosCredentialCache KerberosCredentialType = "ccache"
	// KerberosCredentialPassword derives the principal's key from a password.
	KerberosCredentialPassword KerberosCredentialType = "password"
)

var (
	errBadDataSource       = errors.New("datasource is invalid")
	errBadEndpoint         = errors.New("endpoint must be specified as host:port")
	errBadPort             = errors.New("invalid port in endpoint")
	errEmptyEndpoint       = errors.New("endpoint must be specified")
	errEmptyPassword       = errors.New("password must be set")
	errEmptyService        = errors.New("service must be specified")
	errEmptyUsername       = errors.New("username must be set")
	errMaxQuerySampleCount = errors.New("`max_query_sample_count` must be between 1 and 10000")
	errTopQueryCount       = errors.New("`top_query_count` must be between 1 and 200 and less than or equal to `max_query_sample_count`")

	errInvalidAuthType         = errors.New("`auth_type` must be either 'password' or 'kerberos'")
	errMissingKerberosBlock    = errors.New("`kerberos` block must be set when `auth_type` is 'kerberos'")
	errInvalidCredentialType   = errors.New("`kerberos::credential_type` must be one of 'keytab', 'ccache', or 'password'")
	errEmptyKerberosRealm      = errors.New("`kerberos::realm` must be set")
	errEmptyKerberosPrincipal  = errors.New("`kerberos::principal` must be set")
	errKerberosPrincipalRealm  = errors.New("`kerberos::principal` must not include a realm (no '@'); set the realm in `kerberos::realm`")
	errEmptyKerberosConfigFile = errors.New("`kerberos::config_file` must be set")
	errEmptyKerberosKeytab     = errors.New("`kerberos::keytab_file` must be set when `credential_type` is 'keytab'")
	errEmptyKerberosCredCache  = errors.New("`kerberos::credential_cache` must be set when `credential_type` is 'ccache'")
	errEmptyKerberosPassword   = errors.New("`kerberos::password` must be set when `credential_type` is 'password'")
	errUnexpectedKerberosBlock = errors.New("`kerberos` block must not be set unless `auth_type` is 'kerberos'")
	errKerberosDataSourceCreds = errors.New("`datasource` must not contain a username, password, or `AUTH TYPE` when `auth_type` is 'kerberos'; the identity is derived from the Kerberos principal")
	errUnexpectedConnectorType = errors.New("go-ora did not return an OracleConnector")
)

type TopQueryCollection struct {
	MaxQuerySampleCount uint          `mapstructure:"max_query_sample_count"`
	TopQueryCount       uint          `mapstructure:"top_query_count"`
	CollectionInterval  time.Duration `mapstructure:"collection_interval"`
	AllowedCommentKeys  []string      `mapstructure:"allowed_comment_keys"`
}

type QuerySample struct {
	MaxRowsPerQuery    uint64   `mapstructure:"max_rows_per_query"`
	AllowedCommentKeys []string `mapstructure:"allowed_comment_keys"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type SessionWaitEvent struct {
	MaxRowsPerQuery uint64 `mapstructure:"max_rows_per_query"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// KerberosConfig configures Kerberos (GSSAPI) authentication to the Oracle
// database. It applies only when the top-level `auth_type` is set to
// `kerberos`. The receiver acquires a Kerberos ticket-granting ticket from the
// configured credential source and presents an AP-REQ to the database over
// Oracle Net's advanced negotiation layer; no username/password is sent.
type KerberosConfig struct {
	// CredentialType selects how the Kerberos client obtains its credentials:
	// `keytab`, `ccache`, or `password`.
	CredentialType KerberosCredentialType `mapstructure:"credential_type"`
	// Realm is the Kerberos realm of the principal (e.g. EXAMPLE.COM).
	Realm string `mapstructure:"realm"`
	// Principal is the client principal name, without the realm (e.g. otel).
	Principal string `mapstructure:"principal"`
	// ConfigFile is the path to the krb5.conf configuration file.
	ConfigFile string `mapstructure:"config_file"`
	// KeytabFile is the path to the keytab file. Used when credential_type is `keytab`.
	KeytabFile string `mapstructure:"keytab_file"`
	// CredentialCache is the path to the credential cache. Used when credential_type is `ccache`.
	CredentialCache string `mapstructure:"credential_cache"`
	// Password is the principal's password. Used when credential_type is `password`.
	Password configopaque.String `mapstructure:"password"`
	// DisableFASTNegotiation disables PA-FX-FAST pre-authentication. Set to true
	// when the KDC does not support FAST.
	DisableFASTNegotiation bool `mapstructure:"disable_fast_negotiation"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type Config struct {
	DataSource           string                         `mapstructure:"datasource"`
	Endpoint             string                         `mapstructure:"endpoint"`
	Password             string                         `mapstructure:"password"`
	Service              string                         `mapstructure:"service"`
	Username             string                         `mapstructure:"username"`
	AuthType             AuthType                       `mapstructure:"auth_type"`
	Kerberos             *KerberosConfig                `mapstructure:"kerberos"`
	ControllerConfig     scraperhelper.ControllerConfig `mapstructure:",squash"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	LogsBuilderConfig    metadata.LogsBuilderConfig     `mapstructure:",squash"`

	TopQueryCollection TopQueryCollection `mapstructure:"top_query_collection"`
	QuerySample        QuerySample        `mapstructure:"query_sample_collection"`
	SessionWaitEvent   SessionWaitEvent   `mapstructure:"session_wait_event_collection"`
}

func (c Config) Validate() error {
	var allErrs error

	// auth_type defaults to password when unset.
	authType := c.AuthType
	if authType == "" {
		authType = AuthTypePassword
	}
	if authType != AuthTypePassword && authType != AuthTypeKerberos {
		allErrs = multierr.Append(allErrs, errInvalidAuthType)
	}

	kerberos := authType == AuthTypeKerberos
	if kerberos {
		allErrs = multierr.Append(allErrs, c.validateKerberos())
	} else if c.Kerberos != nil {
		allErrs = multierr.Append(allErrs, errUnexpectedKerberosBlock)
	}

	// If DataSource is defined it takes precedence over the rest of the connection options.
	if c.DataSource == "" {
		if c.Endpoint == "" {
			allErrs = multierr.Append(allErrs, errEmptyEndpoint)
		}

		host, portStr, err := net.SplitHostPort(c.Endpoint)
		if err != nil {
			return multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadEndpoint, err.Error()))
		}

		if host == "" {
			allErrs = multierr.Append(allErrs, errBadEndpoint)
		}

		port, err := strconv.ParseInt(portStr, 10, 32)
		if err != nil {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadPort, err.Error()))
		}

		if port < 0 || port > 65535 {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %d", errBadPort, port))
		}

		// Kerberos authenticates via an external ticket; no username/password is sent.
		if !kerberos {
			if c.Username == "" {
				allErrs = multierr.Append(allErrs, errEmptyUsername)
			}

			if c.Password == "" {
				allErrs = multierr.Append(allErrs, errEmptyPassword)
			}
		}

		if c.Service == "" {
			allErrs = multierr.Append(allErrs, errEmptyService)
		}
	} else {
		u, err := url.Parse(c.DataSource)
		if err != nil {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadDataSource, err.Error()))
		} else if kerberos {
			// With Kerberos the identity comes from the principal, so the data
			// source must not embed a username/password or its own auth type.
			_, hasPassword := u.User.Password()
			hasAuthType := false
			for key := range u.Query() {
				// go-ora matches query keys case-insensitively.
				if strings.EqualFold(key, "AUTH TYPE") {
					hasAuthType = true
					break
				}
			}
			if u.User.Username() != "" || hasPassword || hasAuthType {
				allErrs = multierr.Append(allErrs, errKerberosDataSourceCreds)
			}
		}
	}

	if c.TopQueryCollection.MaxQuerySampleCount < 1 || c.TopQueryCollection.MaxQuerySampleCount > 10000 {
		allErrs = multierr.Append(allErrs, errMaxQuerySampleCount)
	}
	if c.TopQueryCollection.TopQueryCount < 1 || c.TopQueryCollection.TopQueryCount > 200 || c.TopQueryCollection.TopQueryCount > c.TopQueryCollection.MaxQuerySampleCount {
		allErrs = multierr.Append(allErrs, errTopQueryCount)
	}
	return allErrs
}

// validateKerberos checks the kerberos block. It requires the block to be
// present and validates the fields required by the selected credential type.
//
// config_file is the only field common to every credential type. realm and
// principal are required for the keytab and password types, which build the
// client from those values; the ccache type reads the client principal and
// realm from the cache file itself, so it does not need them.
func (c Config) validateKerberos() error {
	if c.Kerberos == nil {
		return errMissingKerberosBlock
	}
	var errs error
	k := c.Kerberos

	if k.ConfigFile == "" {
		errs = multierr.Append(errs, errEmptyKerberosConfigFile)
	}

	switch k.CredentialType {
	case KerberosCredentialKeytab:
		errs = multierr.Append(errs, validateKerberosPrincipalRealm(k))
		if k.KeytabFile == "" {
			errs = multierr.Append(errs, errEmptyKerberosKeytab)
		}
	case KerberosCredentialCache:
		if k.CredentialCache == "" {
			errs = multierr.Append(errs, errEmptyKerberosCredCache)
		}
	case KerberosCredentialPassword:
		errs = multierr.Append(errs, validateKerberosPrincipalRealm(k))
		if k.Password == "" {
			errs = multierr.Append(errs, errEmptyKerberosPassword)
		}
	default:
		errs = multierr.Append(errs, errInvalidCredentialType)
	}

	return errs
}

// validateKerberosPrincipalRealm validates the realm and principal fields,
// which are required by the credential types that build the Kerberos client
// from them (keytab and password).
func validateKerberosPrincipalRealm(k *KerberosConfig) error {
	var errs error
	if k.Realm == "" {
		errs = multierr.Append(errs, errEmptyKerberosRealm)
	}
	if k.Principal == "" {
		errs = multierr.Append(errs, errEmptyKerberosPrincipal)
	} else if strings.Contains(k.Principal, "@") {
		errs = multierr.Append(errs, errKerberosPrincipalRealm)
	}
	return errs
}
