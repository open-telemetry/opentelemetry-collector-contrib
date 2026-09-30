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

	defaultBaoMountPath        = "secret"
	defaultDeadLetterKeyPrefix = "dead_letter/"
)

var (
	errInvalidMode            = errors.New("mode must be sync")
	errInvalidFailureMode     = errors.New("failure_mode must be strict or mark")
	errInvalidKeySourceType   = errors.New("key_source.type must be k8s_secret, env, file, or bao")
	errMissingKeySourceConfig = errors.New("key_source config block is missing for the specified type")
	errKeySourceNeedsMaterial = errors.New("key_source must provide a certificate or hmac_key")
	errDeadLetterNeedsStorage = errors.New("dead_letter.enabled requires dead_letter.storage")
	errInvalidDeadLetterMode  = errors.New("dead_letter.failure_modes entries must be strict or mark")
)

type Config struct {
	Mode                string           `mapstructure:"mode"`
	FailureMode         string           `mapstructure:"failure_mode"`
	VerificationProfile string           `mapstructure:"verification_profile"`
	KeySource           KeySourceConfig  `mapstructure:"key_source"`
	DeadLetter          DeadLetterConfig `mapstructure:"dead_letter"`
}

type KeySourceConfig struct {
	Type      string           `mapstructure:"type"`
	Env       *EnvKeyConfig    `mapstructure:"env"`
	File      *FileKeyConfig   `mapstructure:"file"`
	K8sSecret *K8sSecretConfig `mapstructure:"k8s_secret"`
	Bao       *BaoKeyConfig    `mapstructure:"bao"`
}

// SecretConfig holds verification key material fields shared by providers.
// Set Certificate or HMACKey (no private-key fields; verify only needs
// public material or an HMAC secret).
type SecretConfig struct {
	Certificate string `mapstructure:"certificate"`
	HMACKey     string `mapstructure:"hmac_key"`
}

// EnvKeyConfig configures inline key material whose values are resolved by the
// confmap provider (e.g. via ${env:VAR_NAME} substitution in the collector
// config). Each field holds the actual PEM text or HMAC secret, not an
// env-var name.
type EnvKeyConfig SecretConfig

// FileKeyConfig configures file-based key material (local paths).
type FileKeyConfig SecretConfig

// K8sSecretConfig configures a Kubernetes Secret key source.
// certificate and hmac_key are keys within the Secret data.
type K8sSecretConfig struct {
	Name         string `mapstructure:"name"`
	Namespace    string `mapstructure:"namespace"`
	SecretConfig `mapstructure:",squash"`
}

// BaoKeyConfig configures the OpenBao (Vault-compatible) key material source.
// Address and Token are optional: if omitted, the client reads BAO_ADDR and
// BAO_TOKEN (or any other supported BAO_* environment variables) automatically.
// MountPath is the KV v2 engine mount point (default: "secret").
// SecretPath is the path to the secret within that mount (e.g. "verify").
// certificate and hmac_key are field names within the secret at SecretPath.
type BaoKeyConfig struct {
	Address      string `mapstructure:"address"`
	Token        string `mapstructure:"token"`
	MountPath    string `mapstructure:"mount_path"`
	SecretPath   string `mapstructure:"secret_path"`
	SecretConfig `mapstructure:",squash"`
}

type DeadLetterConfig struct {
	Enabled               bool          `mapstructure:"enabled"`
	StorageID             component.ID  `mapstructure:"storage"`
	KeyPrefix             string        `mapstructure:"key_prefix"`
	IncludeRecord         *bool         `mapstructure:"include_record"`
	IncludeResource       *bool         `mapstructure:"include_resource"`
	Reasons               []string      `mapstructure:"reasons"`
	FailureModes          []string      `mapstructure:"failure_modes"`
	// TODO: validate bounds when DLQ is implemented (e.g. >0 and an upper cap).
	MaxEntrySizeBytes     int           `mapstructure:"max_entry_size_bytes"`
	FailOnStorageError    *bool         `mapstructure:"fail_on_storage_error"`
	PartitionByStream     bool          `mapstructure:"partition_by_stream"`
	DeduplicateByRecordID bool          `mapstructure:"deduplicate_by_record_id"`
	MaintainIndex         bool          `mapstructure:"maintain_index"`
	TTL                   time.Duration `mapstructure:"ttl"`
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
		return validateSecretMaterial(c.KeySource.K8sSecret.SecretConfig)
	case keySourceEnv:
		if c.KeySource.Env == nil {
			return errMissingKeySourceConfig
		}
		return validateSecretMaterial(SecretConfig(*c.KeySource.Env))
	case keySourceFile:
		if c.KeySource.File == nil {
			return errMissingKeySourceConfig
		}
		return validateSecretMaterial(SecretConfig(*c.KeySource.File))
	case keySourceBao:
		if c.KeySource.Bao == nil {
			return errMissingKeySourceConfig
		}
		if c.KeySource.Bao.MountPath == "" {
			c.KeySource.Bao.MountPath = defaultBaoMountPath
		}
		if c.KeySource.Bao.SecretPath == "" {
			return errors.New("key_source.bao.secret_path is required")
		}
		return validateSecretMaterial(c.KeySource.Bao.SecretConfig)
	default:
		return errInvalidKeySourceType
	}
}

func validateSecretMaterial(sc SecretConfig) error {
	if sc.Certificate == "" && sc.HMACKey == "" {
		return errKeySourceNeedsMaterial
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
