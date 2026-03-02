// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errInvalidEndpoint   = errors.New(`"endpoint" must be in the form of <hostname>:<port>`)
	errInvalidFileFormat = errors.New(`"file_format" must be one of: auto, pem, jks, pkcs12`)
)

// FileFormat represents the format of a local certificate file.
type FileFormat string

const (
	// FileFormatAuto infers the format from the file extension.
	FileFormatAuto FileFormat = "auto"
	// FileFormatPEM indicates a PEM-encoded certificate file.
	FileFormatPEM FileFormat = "pem"
	// FileFormatJKS indicates a Java KeyStore file.
	FileFormatJKS FileFormat = "jks"
	// FileFormatPKCS12 indicates a PKCS#12 / PFX keystore file.
	FileFormatPKCS12 FileFormat = "pkcs12"
)

// CertificateTarget represents a target for certificate checking, which can be either
// a network endpoint or a local file
type CertificateTarget struct {
	confignet.TCPAddrConfig `mapstructure:",squash"`
	FilePath                string              `mapstructure:"file_path"`
	FileFormat              FileFormat          `mapstructure:"file_format"`
	Password                configopaque.String `mapstructure:"password"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*CertificateTarget `mapstructure:"targets"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func validateTarget(ct *CertificateTarget) error {
	if ct.Endpoint != "" && ct.FilePath != "" {
		return errors.New("cannot specify both endpoint and file_path")
	}
	if ct.Endpoint == "" && ct.FilePath == "" {
		return errors.New("must specify either endpoint or file_path")
	}

	if ct.FilePath != "" {
		switch ct.FileFormat {
		case FileFormatAuto, FileFormatPEM, FileFormatJKS, FileFormatPKCS12, "":
			// valid — "" is treated the same as "auto"
		default:
			return fmt.Errorf("%w: got %q", errInvalidFileFormat, ct.FileFormat)
		}
	} else {
		// Endpoint-based target: file-related options must not be set.
		if ct.FileFormat != "" {
			return errors.New(`"file_format" cannot be set when "file_path" is empty (endpoint-based target)`)
		}
		if ct.Password != "" {
			return errors.New(`"password" cannot be set when "file_path" is empty (endpoint-based target)`)
		}
	}

	return nil
}

func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, errMissingTargets)
	}

	for _, target := range cfg.Targets {
		err = multierr.Append(err, validateTarget(target))
	}

	return err
}
