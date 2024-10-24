// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gelfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// Config defines configuration for file exporter.
type Config struct {
	Endpoint        string `mapstructure:"endpoint"`
	Protocol        string `mapstructure:"protocol,omitempty"`
	CompressionType string `mapstructure:"compression,omitempty"`
	ChunksOverflow  bool   `mapstructure:"send_chunks_with_overflow,omitempty"`
	ChunkSize       int    `mapstructure:"chunk_size,omitempty"`
	Hostname        string `mapstructure:"hostname,omitempty"`
	TCPTimeout      int    `mapstructure:"tcp_timeout_millis,omitempty"`
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
	}else{
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

	return nil
}
