// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"

import (
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

const (
	idleConnTimeout                = 30
	remoteProxyMaxIdleConnsPerHost = 2
)

// Config is the configuration for the local TCP proxy server.
type Config struct {
	// endpoint is the TCP address and port on which this receiver listens for
	// calls from the X-Ray SDK and relays them to the AWS X-Ray backend to
	// get sampling rules and report sampling statistics.
	confignet.TCPAddr `mapstructure:",squash"`

	// ProxyAddress defines the proxy address that the local TCP server
	// forwards HTTP requests to AWS X-Ray backend through.
	ProxyAddress string `mapstructure:"proxy_address"`

	// TLSSetting struct exposes TLS client configuration when forwarding
	// calls to the AWS X-Ray backend.
	TLSSetting configtls.TLSClientSetting `mapstructure:"tls,omitempty"`

	// Region is the AWS region the local TCP server forwards requests to.
	Region string `mapstructure:"region"`

	// RoleARN is the IAM role used by the local TCP server when
	// communicating with the AWS X-Ray service.
	RoleARN string `mapstructure:"role_arn"`

	// AWSEndpoint is the X-Ray service endpoint which the local
	// TCP server forwards requests to.
	AWSEndpoint string `mapstructure:"aws_endpoint"`

	// LocalMode determines whether the EC2 instance metadata endpoint
	// will be called or not. Set to `true` to skip EC2 instance
	// metadata check.
	LocalMode bool `mapstructure:"local_mode"`

	// Change the default profile for shared creds file
	Profile string `mapstructure:"profile"`

	// Change the default shared creds file location
	SharedCredentialsFile []string `mapstructure:"shared_credentials_file"`

	// Add a custom certificates file
	CertificateFilePath string `mapstructure:"certificate_file_path"`

	// How many times should we retry imds v2
	IMDSRetries int `mapstructure:"imds_retries"`
}

func DefaultConfig() *Config {
	return &Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "0.0.0.0:2000",
		},
		ProxyAddress: "",
		TLSSetting: configtls.TLSClientSetting{
			Insecure:   false,
			ServerName: "",
		},
		Region:      "",
		RoleARN:     "",
		AWSEndpoint: "",
	}
}

func (cfg *Config) toSessionConfig() *awsutil.AWSSessionSettings {
	sessionSettings := awsutil.CreateDefaultSessionConfig()
	sessionSettings.CertificateFilePath = cfg.CertificateFilePath
	sessionSettings.Endpoint = cfg.AWSEndpoint
	sessionSettings.IMDSRetries = cfg.IMDSRetries
	sessionSettings.LocalMode = cfg.LocalMode
	sessionSettings.MaxRetries = remoteProxyMaxIdleConnsPerHost
	sessionSettings.Profile = cfg.Profile
	sessionSettings.ProxyAddress = cfg.ProxyAddress
	sessionSettings.Region = cfg.Region
	sessionSettings.RequestTimeoutSeconds = idleConnTimeout
	sessionSettings.RoleARN = cfg.RoleARN
	sessionSettings.SharedCredentialsFile = cfg.SharedCredentialsFile
	return &sessionSettings
}
