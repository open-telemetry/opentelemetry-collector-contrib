// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

const (
	defaultMode        = "sync"
	defaultFailureMode = "strict"
	defaultProfile     = "default"

	modeSync = "sync"

	failureModeStrict = "strict"
	failureModeMark   = "mark"

	keySourceK8sSecret = "k8s_secret"
	keySourceEnv       = "env"
	keySourceFile      = "file"
	keySourceBao       = "bao"

	defaultDeadLetterKeyPrefix = "dead_letter/"
)

var (
	errInvalidMode            = errors.New("mode must be sync")
	errInvalidFailureMode     = errors.New("failure_mode must be strict or mark")
	errInvalidKeySourceType   = errors.New("key_source.type must be k8s_secret, env, file, or bao")
	errMissingKeySourceConfig = errors.New("key_source config block is missing for the specified type")
	errKeySourceNeedsMaterial = errors.New("key_source must provide a certificate and/or HMAC key for integrity verification")
	errHashChainNeedsStorage  = errors.New("hash_chain.enabled requires hash_chain.storage")
	errDeadLetterNeedsStorage = errors.New("dead_letter.enabled requires dead_letter.storage")
	errInvalidDeadLetterMode  = errors.New("dead_letter.failure_modes entries must be strict or mark")
)

type Config struct {
	Mode                string           `mapstructure:"mode"`
	FailureMode         string           `mapstructure:"failure_mode"`
	VerificationProfile string           `mapstructure:"verification_profile"`
	KeySource           KeySourceConfig  `mapstructure:"key_source"`
	HashChain           HashChainConfig  `mapstructure:"hash_chain"`
	DeadLetter          DeadLetterConfig `mapstructure:"dead_letter"`
}

type KeySourceConfig struct {
	Type      string           `mapstructure:"type"`
	K8sSecret *K8sSecretConfig `mapstructure:"k8s_secret"`
	Env       *EnvKeyConfig    `mapstructure:"env"`
	File      *FileKeyConfig   `mapstructure:"file"`
	Bao       *BaoKeyConfig    `mapstructure:"bao"`
}

// K8sSecretConfig configures a Kubernetes Secret key source.
// certificate and hmac_key are keys within the Secret data.
// Both may be set when the collector should verify either algorithm.
type K8sSecretConfig struct {
	Name      string `mapstructure:"name"`
	Namespace string `mapstructure:"namespace"`
	CertKey   string `mapstructure:"certificate"`
	HMACKey   string `mapstructure:"hmac_key"`
}

// EnvKeyConfig configures environment-variable key material.
// certificate and hmac_key are the names of environment variables that hold
// the PEM certificate and/or HMAC secret.
// Both may be set when the collector should verify either algorithm.
type EnvKeyConfig struct {
	CertEnvVar    string `mapstructure:"certificate"`
	HMACKeyEnvVar string `mapstructure:"hmac_key"`
}

// FileKeyConfig configures file-based key material.
// certificate and hmac_key are local file paths.
// Both may be set when the collector should verify either algorithm.
type FileKeyConfig struct {
	CertFile    string `mapstructure:"certificate"`
	HMACKeyFile string `mapstructure:"hmac_key"`
}

// BaoKeyConfig configures the OpenBao (Vault-compatible) key material source.
// Address and Token are optional: if omitted, the client reads BAO_ADDR and
// BAO_TOKEN (or any other supported BAO_* environment variables) automatically.
// certificate and hmac_key are field names within the secret at SecretPath.
// Both may be set when the collector should verify either algorithm.
type BaoKeyConfig struct {
	Address      string `mapstructure:"address"`
	Token        string `mapstructure:"token"`
	SecretPath   string `mapstructure:"secret_path"`
	CertField    string `mapstructure:"certificate"`
	HMACKeyField string `mapstructure:"hmac_key"`
}

type DeadLetterConfig struct {
	Enabled               bool          `mapstructure:"enabled"`
	StorageID             component.ID  `mapstructure:"storage"`
	KeyPrefix             string        `mapstructure:"key_prefix"`
	IncludeRecord         *bool         `mapstructure:"include_record"`
	IncludeResource       *bool         `mapstructure:"include_resource"`
	Reasons               []string      `mapstructure:"reasons"`
	FailureModes          []string      `mapstructure:"failure_modes"`
	MaxEntrySizeBytes     int           `mapstructure:"max_entry_size_bytes"`
	FailOnStorageError    *bool         `mapstructure:"fail_on_storage_error"`
	PartitionByStream     bool          `mapstructure:"partition_by_stream"`
	DeduplicateByRecordID bool          `mapstructure:"deduplicate_by_record_id"`
	MaintainIndex         bool          `mapstructure:"maintain_index"`
	TTL                   time.Duration `mapstructure:"ttl"`
}

type HashChainConfig struct {
	Enabled   bool         `mapstructure:"enabled"`
	StorageID component.ID `mapstructure:"storage"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Mode:                defaultMode,
		FailureMode:         defaultFailureMode,
		VerificationProfile: defaultProfile,
	}
}

func (c *Config) Validate() error {
	if c.Mode == "" {
		c.Mode = defaultMode
	} else if c.Mode != modeSync {
		return errInvalidMode
	}

	if c.FailureMode == "" {
		c.FailureMode = defaultFailureMode
	} else if c.FailureMode != failureModeStrict && c.FailureMode != failureModeMark {
		return errInvalidFailureMode
	}

	if c.VerificationProfile == "" {
		c.VerificationProfile = defaultProfile
	}

	if err := c.validateKeySource(); err != nil {
		return err
	}

	if c.HashChain.Enabled && c.HashChain.StorageID == (component.ID{}) {
		return errHashChainNeedsStorage
	}

	return c.DeadLetter.validate()
}

func (c *Config) validateKeySource() error {
	switch c.KeySource.Type {
	case keySourceK8sSecret:
		if c.KeySource.K8sSecret == nil {
			return errMissingKeySourceConfig
		}
		if c.KeySource.K8sSecret.Name == "" {
			return errors.New("key_source.k8s_secret.name is required")
		}
		if c.KeySource.K8sSecret.Namespace == "" {
			c.KeySource.K8sSecret.Namespace = "default"
		}
		if c.KeySource.K8sSecret.CertKey == "" && c.KeySource.K8sSecret.HMACKey == "" {
			return errKeySourceNeedsMaterial
		}
	case keySourceEnv:
		if c.KeySource.Env == nil {
			return errMissingKeySourceConfig
		}
		if c.KeySource.Env.CertEnvVar == "" && c.KeySource.Env.HMACKeyEnvVar == "" {
			return errKeySourceNeedsMaterial
		}
	case keySourceFile:
		if c.KeySource.File == nil {
			return errMissingKeySourceConfig
		}
		if c.KeySource.File.CertFile == "" && c.KeySource.File.HMACKeyFile == "" {
			return errKeySourceNeedsMaterial
		}
	case keySourceBao:
		if c.KeySource.Bao == nil {
			return errMissingKeySourceConfig
		}
		if c.KeySource.Bao.SecretPath == "" {
			return errors.New("key_source.bao.secret_path is required")
		}
		if c.KeySource.Bao.CertField == "" && c.KeySource.Bao.HMACKeyField == "" {
			return errKeySourceNeedsMaterial
		}
	default:
		return errInvalidKeySourceType
	}
	return nil
}

func (dl *DeadLetterConfig) validate() error {
	if !dl.Enabled {
		return nil
	}
	if dl.StorageID == (component.ID{}) {
		return errDeadLetterNeedsStorage
	}
	if dl.KeyPrefix == "" {
		dl.KeyPrefix = defaultDeadLetterKeyPrefix
	}
	for _, mode := range dl.FailureModes {
		if mode != failureModeStrict && mode != failureModeMark {
			return errInvalidDeadLetterMode
		}
	}
	return nil
}

var _ component.Config = (*Config)(nil)
