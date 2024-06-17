package gelfreceiver

import (
	"fmt"
	"net"
	"strings"
)

// Config represents the receiver config settings within the collector's config.yaml
type Config struct {
	ListenAddress string `mapstructure:"listen_address,omitempty"`
	Protocol      string `mapstructure:"protocol,omitempty"`
	LogBuffer     int `mapstructure:"log_message_buffer,omitempty"`
}

// Validate checks if the receiver configuration is valid
func (cfg *Config) Validate() error {

	fmt.Println("Config validation in-progress")

	// 1. Listen Address Validation
	if _, _, err := net.SplitHostPort(cfg.ListenAddress); err != nil {
		return fmt.Errorf("invalid listen_address: %w", err)
	}

	// 2. Protocol Validation
	protocol := strings.ToLower(cfg.Protocol)
	if protocol != "udp" && protocol != "tcp" {
		return fmt.Errorf("invalid protocol: must be 'udp' or 'tcp'")
	}

	// 3. Success (No Errors)
	return nil
}
