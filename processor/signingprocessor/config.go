// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"crypto"
	"errors"

	"go.opentelemetry.io/collector/component"
)

const (
	defaultAlgorithm      = "RS256"
	defaultCertificateRef = "fingerprint"

	// Algorithm constants — JWA identifiers (RFC 7518 / RFC 8037 / IANA).
	AlgorithmRS256      = "RS256"
	AlgorithmRS512      = "RS512"
	AlgorithmES256      = "ES256"
	AlgorithmEdDSA      = "EdDSA"
	AlgorithmHMACSHA256 = "HMAC-SHA256"

	KeySourceK8sSecret = "k8s_secret"
	KeySourceEnv       = "env"
	KeySourceFile      = "file"
	KeySourceBao       = "bao"

	CertificateRefFingerprint = "fingerprint"
	CertificateRefFull        = "full"
)

var (
	errInvalidAlgorithm       = errors.New("algorithm must be RS256, RS512, ES256, EdDSA, or HMAC-SHA256")
	errInvalidKeySourceType   = errors.New("key_source.type must be k8s_secret, env, file, or bao")
	errMissingKeySourceConfig = errors.New("key_source config block is missing for the specified type")
	errInvalidCertificateRef  = errors.New("certificate_ref must be fingerprint or full")
	errHMACNoCertRef          = errors.New("certificate_ref must not be set for HMAC-SHA256 (symmetric algorithm has no certificate)")
)

type Config struct {
	// Algorithm is the JWA signing algorithm.
	// Valid values: RS256, RS512, ES256, EdDSA, HMAC-SHA256. Default: RS256.
	Algorithm string `mapstructure:"algorithm"`
	// CertificateRef controls how the certificate is encoded in the
	// audit.integrity.certificate resource attribute. Not used for HMAC-SHA256.
	CertificateRef string          `mapstructure:"certificate_ref"`
	KeySource      KeySourceConfig `mapstructure:"key_source"`
}

type KeySourceConfig struct {
	Type      string           `mapstructure:"type"`
	K8sSecret *K8sSecretConfig `mapstructure:"k8s_secret"`
	Env       *EnvKeyConfig    `mapstructure:"env"`
	File      *FileKeyConfig   `mapstructure:"file"`
	Bao       *BaoKeyConfig    `mapstructure:"bao"`
}

// K8sSecretConfig configures a Kubernetes Secret key source.
// For asymmetric algorithms set CertKey and KeyKey.
// For HMAC-SHA256 set HMACKey instead.
type K8sSecretConfig struct {
	Name      string `mapstructure:"name"`
	Namespace string `mapstructure:"namespace"`
	// Asymmetric key fields
	CertKey string `mapstructure:"cert_key"`
	KeyKey  string `mapstructure:"key_key"`
	// HMAC-SHA256 field
	HMACKey string `mapstructure:"hmac_key"`
}

// EnvKeyConfig configures environment-variable key material.
// For asymmetric algorithms set CertEnvVar and KeyEnvVar.
// For HMAC-SHA256 set HMACKeyEnvVar instead.
type EnvKeyConfig struct {
	// Asymmetric key fields
	CertEnvVar string `mapstructure:"cert_env_var"`
	KeyEnvVar  string `mapstructure:"key_env_var"`
	// HMAC-SHA256 field
	HMACKeyEnvVar string `mapstructure:"hmac_key_env_var"`
}

// FileKeyConfig configures file-based key material.
// For asymmetric algorithms set CertFile and KeyFile.
// For HMAC-SHA256 set HMACKeyFile instead.
type FileKeyConfig struct {
	// Asymmetric key fields
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	// HMAC-SHA256 field
	HMACKeyFile string `mapstructure:"hmac_key_file"`
}

// BaoKeyConfig configures the OpenBao (Vault-compatible) key material source.
// Address and Token are optional: if omitted, the client reads BAO_ADDR and
// BAO_TOKEN (or any other supported BAO_* environment variables) automatically.
// For asymmetric algorithms set CertField and KeyField.
// For HMAC-SHA256 set HMACKeyField instead.
type BaoKeyConfig struct {
	Address    string `mapstructure:"address"`
	Token      string `mapstructure:"token"`
	SecretPath string `mapstructure:"secret_path"`
	// Asymmetric key fields
	CertField string `mapstructure:"cert_field"`
	KeyField  string `mapstructure:"key_field"`
	// HMAC-SHA256 field
	HMACKeyField string `mapstructure:"hmac_key_field"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Algorithm:      defaultAlgorithm,
		CertificateRef: defaultCertificateRef,
	}
}

func (c *Config) Validate() error {
	isHMAC := false
	switch c.Algorithm {
	case AlgorithmRS256, AlgorithmRS512, AlgorithmES256, AlgorithmEdDSA:
		// valid asymmetric
	case AlgorithmHMACSHA256:
		isHMAC = true
	case "":
		c.Algorithm = defaultAlgorithm
	default:
		return errInvalidAlgorithm
	}

	if isHMAC {
		if c.CertificateRef != "" {
			return errHMACNoCertRef
		}
	} else {
		if c.CertificateRef == "" {
			c.CertificateRef = defaultCertificateRef
		} else if c.CertificateRef != CertificateRefFingerprint && c.CertificateRef != CertificateRefFull {
			return errInvalidCertificateRef
		}
	}

	switch c.KeySource.Type {
	case KeySourceK8sSecret:
		if c.KeySource.K8sSecret == nil {
			return errMissingKeySourceConfig
		}
		if c.KeySource.K8sSecret.Name == "" {
			return errors.New("key_source.k8s_secret.name is required")
		}
		if c.KeySource.K8sSecret.Namespace == "" {
			c.KeySource.K8sSecret.Namespace = "default"
		}
		if isHMAC {
			if c.KeySource.K8sSecret.HMACKey == "" {
				return errors.New("key_source.k8s_secret.hmac_key is required for HMAC-SHA256")
			}
		} else {
			if c.KeySource.K8sSecret.CertKey == "" {
				return errors.New("key_source.k8s_secret.cert_key is required")
			}
			if c.KeySource.K8sSecret.KeyKey == "" {
				return errors.New("key_source.k8s_secret.key_key is required")
			}
		}
	case KeySourceEnv:
		if c.KeySource.Env == nil {
			return errMissingKeySourceConfig
		}
		if isHMAC {
			if c.KeySource.Env.HMACKeyEnvVar == "" {
				return errors.New("key_source.env.hmac_key_env_var is required for HMAC-SHA256")
			}
		} else {
			if c.KeySource.Env.CertEnvVar == "" {
				return errors.New("key_source.env.cert_env_var is required")
			}
			if c.KeySource.Env.KeyEnvVar == "" {
				return errors.New("key_source.env.key_env_var is required")
			}
		}
	case KeySourceFile:
		if c.KeySource.File == nil {
			return errMissingKeySourceConfig
		}
		if isHMAC {
			if c.KeySource.File.HMACKeyFile == "" {
				return errors.New("key_source.file.hmac_key_file is required for HMAC-SHA256")
			}
		} else {
			if c.KeySource.File.CertFile == "" {
				return errors.New("key_source.file.cert_file is required")
			}
			if c.KeySource.File.KeyFile == "" {
				return errors.New("key_source.file.key_file is required")
			}
		}
	case KeySourceBao:
		if c.KeySource.Bao == nil {
			return errMissingKeySourceConfig
		}
		if c.KeySource.Bao.SecretPath == "" {
			return errors.New("key_source.bao.secret_path is required")
		}
		if isHMAC {
			if c.KeySource.Bao.HMACKeyField == "" {
				return errors.New("key_source.bao.hmac_key_field is required for HMAC-SHA256")
			}
		} else {
			if c.KeySource.Bao.CertField == "" {
				return errors.New("key_source.bao.cert_field is required")
			}
			if c.KeySource.Bao.KeyField == "" {
				return errors.New("key_source.bao.key_field is required")
			}
		}
	default:
		return errInvalidKeySourceType
	}

	return nil
}

// GetHash returns the crypto.Hash for the configured algorithm.
// Returns crypto.Hash(0) for EdDSA (hashes internally).
func (c *Config) GetHash() crypto.Hash {
	switch c.Algorithm {
	case AlgorithmRS512:
		return crypto.SHA512
	case AlgorithmEdDSA:
		return crypto.Hash(0)
	default: // RS256, ES256, HMAC-SHA256
		return crypto.SHA256
	}
}

var _ component.Config = (*Config)(nil)
