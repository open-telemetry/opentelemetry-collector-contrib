// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	if c.Client == nil || c.Client.Conn == nil {
		return nil, fmt.Errorf("SSH client not initialized")
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
func (scs *SSHClientSettings) ToClient(host component.Host, settings component.TelemetrySettings) (*Client, error) {
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
		hkc = ssh.InsecureIgnoreHostKey() //#nosec G106
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
	path := fmt.Sprintf("%s/.ssh/known_hosts", home)
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
