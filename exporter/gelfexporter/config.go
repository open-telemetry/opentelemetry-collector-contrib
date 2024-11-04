// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gelfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type AllowRawGELFMessage struct {
	Enabled                    bool   `mapstructure:"enabled,omitempty"`
	RawGelfMessageAttributeKey string `mapstructure:"raw_gelf_message_attribute_key,omitempty"`
	IgnoreAttributesWithPrefix string `mapstructure:"remove_attributes_with_prefix,omitempty"`
}

type UseGELFAttributes struct {
	Enabled                     bool   `mapstructure:"enabled,omitempty"`
	AttributesPrefix            string `mapstructure:"attributes_prefix,omitempty"`
	ExtractStandardAttributes   bool   `mapstructure:"extract_standard_attributes,omitempty"`
	ExtractAdditionalAttributes bool   `mapstructure:"extract_additional_attributes,omitempty"`
}

type FeatureFlags struct {
	AllowRawGELFMessage AllowRawGELFMessage `mapstructure:"allow_raw_gelf_message,omitempty"`
	UseGELFAttributes   UseGELFAttributes   `mapstructure:"append_gelf_additional_attributes,omitempty"`
}

// Config defines configuration for file exporter.
type Config struct {
	Endpoint        string       `mapstructure:"endpoint"`
	Protocol        string       `mapstructure:"protocol,omitempty"`
	CompressionType string       `mapstructure:"compression,omitempty"`
	ChunksOverflow  bool         `mapstructure:"send_chunks_with_overflow,omitempty"`
	ChunkSize       int          `mapstructure:"chunk_size,omitempty"`
	Hostname        string       `mapstructure:"hostname,omitempty"`
	TCPTimeout      int          `mapstructure:"tcp_timeout_millis,omitempty"`
	FeatureFlags    FeatureFlags `mapstructure:"feature_flags,omitempty"`
}

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("Endpoint must be non-empty")
	}

	if _, _, err := net.SplitHostPort(cfg.Endpoint); err != nil {
		return fmt.Errorf("Endpoint must be a valid host:port")
	}

	if cfg.Protocol == "" {
		cfg.Protocol = defaultProtocol
	}

	if strings.ToLower(cfg.Protocol) != "udp" && strings.ToLower(cfg.Protocol) != "tcp" {
		cfg.Protocol = strings.ToLower(cfg.Protocol)
		return fmt.Errorf(fmt.Sprintf("Protocol must be either 'udp' or 'tcp', got %s", cfg.Protocol))
	}

	if cfg.Protocol == "tcp" && cfg.TCPTimeout < 0 {
		cfg.TCPTimeout = defaultTCPTimeout
	} else {
		if cfg.TCPTimeout > 30000 {
			return fmt.Errorf("TCP timeout must be less than 30000")
		}
	}

	// Compression types can be zlib, gzip or none
	if cfg.CompressionType == "" {
		cfg.CompressionType = defaultCompressionType
	} else if strings.ToLower(cfg.CompressionType) != "zlib" && strings.ToLower(cfg.CompressionType) != "gzip" && strings.ToLower(cfg.CompressionType) != "none" {
		return fmt.Errorf(fmt.Sprintf("Compression must be either 'zlib', 'gzip' or 'none', got %s", cfg.CompressionType))
	}

	if cfg.ChunkSize < 0 {
		return fmt.Errorf("Chunk size must be greater than 0")
	} else if cfg.ChunkSize == 0 {
		cfg.ChunkSize = defaultChunkSize
	}

	if cfg.Hostname == "" {
		internalHostname, err := os.Hostname()
		if err != nil {
			cfg.Hostname = "localhost"
		} else {
			cfg.Hostname = internalHostname
		}
	}

	if cfg.FeatureFlags.AllowRawGELFMessage.Enabled {
		// Both Raw GELF message and GELF attributes cannot be enabled at the same time, due to conflict in fields
		if cfg.FeatureFlags.UseGELFAttributes.Enabled {
			return fmt.Errorf("Raw GELF message and GELF attributes cannot be enabled at the same time")
		}

		if cfg.FeatureFlags.AllowRawGELFMessage.RawGelfMessageAttributeKey == "" {
			return fmt.Errorf("GELF message attribute key must be non-empty")
		}
	}

	return nil
}
