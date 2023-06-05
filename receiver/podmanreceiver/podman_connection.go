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

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// most of this file has been adopted from https://github.com/containers/podman/blob/main/pkg/bindings/connection.go
// and then simplified to remove things we do not need.

func newPodmanConnection(logger *zap.Logger, endpoint string, sshKey string, sshPassphrase string) (*http.Client, error) {
	_url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	switch _url.Scheme {
	case "unix":
		if !strings.HasPrefix(endpoint, "unix:///") {
			// autofix unix://path_element vs unix:///path_element
			_url.Path = "/" + strings.Join([]string{_url.Host, _url.Path}, "/")
			_url.Host = ""
		}
		return unixConnection(_url), nil
	case "tcp":
		if !strings.HasPrefix(endpoint, "tcp://") {
			return nil, errors.New("tcp URIs should begin with tcp://")
		}
		return tcpConnection(_url), nil
	case "ssh":
		secure, err := strconv.ParseBool(_url.Query().Get("secure"))
		if err != nil {
			secure = false
		}
		return sshConnection(logger, _url, secure, sshKey, sshPassphrase)
	default:
		return nil, fmt.Errorf("unable to create connection. %q is not a supported schema", _url.Scheme)
	}
}

func tcpConnection(_url *url.URL) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("tcp", _url.Host)
			},
			DisableCompression: true,
		},
	}
}

func unixConnection(_url *url.URL) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", _url.Path)
			},
			DisableCompression: true,
		},
	}
}

func sshConnection(logger *zap.Logger, _url *url.URL, secure bool, key, passphrase string) (*http.Client, error) {
	var signers []ssh.Signer // order Signers are appended to this list determines which key is presented to server

	if len(key) > 0 {
		s, err := publicKey(key, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("failed to parse identity %q: %w", key, err)
		}

		signers = append(signers, s)
	}

	if sock, found := os.LookupEnv("SSH_AUTH_SOCK"); found {
		c, err := net.Dial("unix", sock)
		if err != nil {
			return nil, err
		}

		agentSigners, err := agent.NewClient(c).Signers()
		if err != nil {
			return nil, err
		}
		signers = append(signers, agentSigners...)
	}

	var authMethods []ssh.AuthMethod
	if len(signers) > 0 {
		var dedup = make(map[string]ssh.Signer)
		// Dedup signers based on fingerprint, ssh-agent keys override CONTAINER_SSHKEY
		for _, s := range signers {
			fp := ssh.FingerprintSHA256(s.PublicKey())
			dedup[fp] = s
		}

		var uniq []ssh.Signer
		for _, s := range dedup {
			uniq = append(uniq, s)
		}
		authMethods = append(authMethods, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return uniq, nil
		}))
	}

	if pw, found := _url.User.Password(); found {
		authMethods = append(authMethods, ssh.Password(pw))
	}

	port := _url.Port()
	if port == "" {
		port = "22"
	}

	callback := ssh.InsecureIgnoreHostKey() // #nosec
	if secure {
		host := _url.Hostname()
		if port != "22" {
			host = fmt.Sprintf("[%s]:%s", host, port)
		}
		key := hostKey(logger, host)
		if key != nil {
			callback = ssh.FixedHostKey(key)
		}
	}

	bastion, err := ssh.Dial("tcp",
		net.JoinHostPort(_url.Hostname(), port),
		&ssh.ClientConfig{
			User:            _url.User.Username(),
			Auth:            authMethods,
			HostKeyCallback: callback,
			HostKeyAlgorithms: []string{
				ssh.KeyAlgoRSA,
				ssh.KeyAlgoDSA,
				ssh.KeyAlgoECDSA256,
				ssh.KeyAlgoECDSA384,
				ssh.KeyAlgoECDSA521,
				ssh.KeyAlgoED25519,
			},
			Timeout: 5 * time.Second,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("connection to bastion host (%s) failed: %w", _url.String(), err)
	}

	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return bastion.Dial("unix", _url.Path)
			},
		},
	}, nil
}

func publicKey(path string, passphrase []byte) (ssh.Signer, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		pmErr := &ssh.PassphraseMissingError{}
		if !errors.As(err, &pmErr) {
			return nil, err
		}
		return ssh.ParsePrivateKeyWithPassphrase(key, passphrase)
	}
	return signer, nil
}

func hostKey(logger *zap.Logger, host string) ssh.PublicKey {
	// parse OpenSSH known_hosts file
	// ssh or use ssh-keyscan to get initial key

	user, err := user.Current()
	if err != nil {
		logger.Error("failed to load known_hosts", zap.Error(err))
		return nil
	}

	knownHosts := filepath.Join(user.HomeDir, ".ssh", "known_hosts")
	fd, err := os.Open(knownHosts)
	if err != nil {
		logger.Error("failed to load known_hosts", zap.Error(err))
		return nil
	}

	// support -H parameter for ssh-keyscan
	hashhost := knownhosts.HashHostname(host)

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		_, hosts, key, _, _, err := ssh.ParseKnownHosts(scanner.Bytes())
		if err != nil {
			logger.Error("Failed to parse known_hosts", zap.String("hosts", scanner.Text()))
			continue
		}

		for _, h := range hosts {
			if h == host || h == hashhost {
				return key
			}
		}
	}

	return nil
}
