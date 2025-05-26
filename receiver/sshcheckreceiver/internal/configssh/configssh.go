// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configssh // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/configssh"

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/pkg/sftp"
	"go.opentelemetry.io/collector/component"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	defaultClientVersion = "SSH-2.0-OTelClient"
)

var errMissingKnownHosts = errors.New(`known_hosts file is missing`)

type SSHClientSettings struct {
	// Endpoint is always required
	Endpoint string        `mapstructure:"endpoint"`
	Timeout  time.Duration `mapstructure:"timeout"`

	// authentication requires a Username and either a Password or KeyFile
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	KeyFile  string `mapstructure:"key_file"`

	// file path to the known_hosts
	KnownHosts string `mapstructure:"known_hosts"`

	// IgnoreHostKey provides an insecure path to quickstarts and testing
	IgnoreHostKey bool `mapstructure:"ignore_host_key"`
}

type Client struct {
	*ssh.Client
	*ssh.ClientConfig
	DialFunc func(network, address string, config *ssh.ClientConfig) (*ssh.Client, error)
}

// Dial starts an SSH session.
func (c *Client) Dial(endpoint string) (err error) {
	c.Client, err = c.DialFunc("tcp", endpoint, c.ClientConfig)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) SFTPClient() (*SFTPClient, error) {
	if c.Client == nil || c.Conn == nil {
		return nil, errors.New("SSH client not initialized")
	}
	client, err := sftp.NewClient(c.Client)
	if err != nil {
		return nil, err
	}
	return &SFTPClient{
		Client:       client,
		ClientConfig: c.ClientConfig,
	}, nil
}

type SFTPClient struct {
	*sftp.Client
	*ssh.ClientConfig
}

// ToClient creates an SSHClient.
func (scs *SSHClientSettings) ToClient(_ component.Host, _ component.TelemetrySettings) (*Client, error) {
	var (
		auth ssh.AuthMethod
		hkc  ssh.HostKeyCallback
	)
	if len(scs.KeyFile) > 0 {
		key, err := os.ReadFile(scs.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read private key: %w", err)
		}

		if len(scs.Password) > 0 {
			sgn, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(scs.Password))
			if err != nil {
				return nil, fmt.Errorf("unable to parse private key with passphrase: %w", err)
			}
			auth = ssh.PublicKeys(sgn)
		} else {
			sgn, err := ssh.ParsePrivateKey(key)
			if err != nil {
				return nil, fmt.Errorf("unable to parse private key with passphrase: %w", err)
			}
			auth = ssh.PublicKeys(sgn)
		}
	} else {
		auth = ssh.Password(scs.Password)
	}

	switch {
	case scs.IgnoreHostKey:
		//nolint:gosec // #nosec G106
		hkc = ssh.InsecureIgnoreHostKey()
	case scs.KnownHosts != "":
		fn, err := knownhosts.New(scs.KnownHosts)
		if err != nil {
			return nil, err
		}
		hkc = fn
	default:
		fn, err := defaultKnownHostsCallback()
		if err != nil {
			return nil, err
		}
		hkc = fn
	}

	return &Client{
		ClientConfig: &ssh.ClientConfig{
			User:            scs.Username,
			Auth:            []ssh.AuthMethod{auth},
			HostKeyCallback: hkc,
			ClientVersion:   defaultClientVersion,
		},
		DialFunc: ssh.Dial,
	}, nil
}

func defaultKnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := home + "/.ssh/known_hosts"
	if _, err := os.Stat(path); err != nil {
		return "", errMissingKnownHosts
	}
	return path, nil
}

func defaultKnownHostsCallback() (hkc ssh.HostKeyCallback, err error) {
	var knownHosts []string
	if homeKH, err := defaultKnownHostsPath(); err == nil {
		knownHosts = append(knownHosts, homeKH)
	}
	if _, err := os.Stat("/etc/ssh/known_hosts"); err == nil {
		knownHosts = append(knownHosts, "/etc/ssh/known_hosts")
	}

	return knownhosts.New(knownHosts...)
}
